package computeruse

import (
	"context"
	"errors"
	"fmt"
	"log"
	"sync"
	"time"
)

var (
	// ErrPoolExhausted는 최대 컨테이너 수에 도달했을 때 반환된다.
	ErrPoolExhausted = errors.New("container pool exhausted: maximum containers reached")

	// ErrPoolShutdown은 풀이 종료된 후 작업 시도 시 반환된다.
	ErrPoolShutdown = errors.New("container pool is shut down")
)

// PoolConfig는 컨테이너 풀 설정을 정의한다.
type PoolConfig struct {
	MaxContainers int           // 최대 컨테이너 수 (기본: 5)
	WarmPoolSize  int           // 워밍 풀 크기 (기본: 2)
	IdleTimeout   time.Duration // 유휴 타임아웃 (기본: 5분)
}

// DefaultPoolConfig는 기본 풀 설정을 반환한다.
func DefaultPoolConfig() PoolConfig {
	return PoolConfig{
		MaxContainers: 5,
		WarmPoolSize:  2,
		IdleTimeout:   5 * time.Minute,
	}
}

// PoolStatus는 풀의 현재 상태를 나타낸다.
type PoolStatus struct {
	WarmCount   int // 워밍 풀에 대기 중인 컨테이너 수
	ActiveCount int // 세션에 할당된 활성 컨테이너 수
	MaxCount    int // 최대 컨테이너 수
}

// activeContainer는 세션에 할당된 활성 컨테이너 정보를 담는다.
type activeContainer struct {
	info       *ContainerInfo
	sessionID  string
	assignedAt time.Time
}

// warmContainer는 워밍 풀에 대기 중인 컨테이너 정보를 담는다.
type warmContainer struct {
	info      *ContainerInfo
	createdAt time.Time
}

// ContainerPool은 웜 컨테이너 풀과 활성 컨테이너를 관리한다.
type ContainerPool struct {
	manager  *ContainerManager
	config   PoolConfig

	warmPool   []warmContainer           // 대기 중인 컨테이너 목록
	activePool map[string]activeContainer // sessionID -> 활성 컨테이너

	shutdown bool
	mu       sync.Mutex
}

// NewContainerPool은 ContainerManager와 설정으로 새 ContainerPool을 생성한다.
func NewContainerPool(manager *ContainerManager, cfg PoolConfig) *ContainerPool {
	if cfg.MaxContainers <= 0 {
		cfg.MaxContainers = DefaultPoolConfig().MaxContainers
	}
	if cfg.WarmPoolSize < 0 {
		cfg.WarmPoolSize = 0
	}
	if cfg.WarmPoolSize > cfg.MaxContainers {
		cfg.WarmPoolSize = cfg.MaxContainers
	}
	if cfg.IdleTimeout <= 0 {
		cfg.IdleTimeout = DefaultPoolConfig().IdleTimeout
	}

	return &ContainerPool{
		manager:    manager,
		config:     cfg,
		warmPool:   make([]warmContainer, 0, cfg.WarmPoolSize),
		activePool: make(map[string]activeContainer),
	}
}

// Acquire는 세션에 컨테이너를 할당한다.
// 워밍 풀에 컨테이너가 있으면 우선 사용하고, 없으면 새로 생성한다.
func (p *ContainerPool) Acquire(ctx context.Context, sessionID string) (*ContainerInfo, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.shutdown {
		return nil, ErrPoolShutdown
	}

	// 이미 할당된 세션인지 확인
	if _, exists := p.activePool[sessionID]; exists {
		return nil, fmt.Errorf("세션 %s에 이미 컨테이너가 할당되어 있습니다", sessionID)
	}

	// 최대 컨테이너 수 확인
	totalCount := len(p.warmPool) + len(p.activePool)
	if totalCount >= p.config.MaxContainers && len(p.warmPool) == 0 {
		return nil, ErrPoolExhausted
	}

	var info *ContainerInfo

	// 워밍 풀에서 컨테이너 가져오기
	if len(p.warmPool) > 0 {
		warm := p.warmPool[0]
		p.warmPool = p.warmPool[1:]
		info = warm.info
		log.Printf("[computer-use] 워밍 풀에서 컨테이너 할당: session=%s, container=%s", sessionID, info.ID[:min(12, len(info.ID))])
	} else {
		// 최대 수 재확인 (워밍 풀에서 가져오지 못한 경우)
		if totalCount >= p.config.MaxContainers {
			return nil, ErrPoolExhausted
		}

		// 새 컨테이너 생성 (잠금 해제 후 생성하지 않음 - 동시성 제어를 위해 잠금 유지)
		p.mu.Unlock()
		var createErr error
		info, createErr = p.manager.Create(ctx)
		p.mu.Lock()

		if createErr != nil {
			return nil, fmt.Errorf("컨테이너 생성 실패: %w", createErr)
		}

		// 잠금 해제 사이에 종료되었는지 확인
		if p.shutdown {
			// 생성된 컨테이너 정리
			_ = p.manager.Remove(ctx, info.ID)
			return nil, ErrPoolShutdown
		}

		log.Printf("[computer-use] 새 컨테이너 생성하여 할당: session=%s, container=%s", sessionID, info.ID[:min(12, len(info.ID))])
	}

	// 활성 풀에 등록
	p.activePool[sessionID] = activeContainer{
		info:       info,
		sessionID:  sessionID,
		assignedAt: time.Now(),
	}

	return info, nil
}

// Release는 세션에서 컨테이너를 해제하고 정리한다.
func (p *ContainerPool) Release(ctx context.Context, sessionID string) error {
	p.mu.Lock()

	active, exists := p.activePool[sessionID]
	if !exists {
		p.mu.Unlock()
		return fmt.Errorf("세션 %s에 할당된 컨테이너가 없습니다", sessionID)
	}

	delete(p.activePool, sessionID)
	containerID := active.info.ID
	p.mu.Unlock()

	// 컨테이너 삭제 (잠금 해제 후 실행 - I/O 작업)
	if err := p.manager.Remove(ctx, containerID); err != nil {
		log.Printf("[computer-use] 컨테이너 삭제 실패: session=%s, container=%s, err=%v",
			sessionID, containerID[:min(12, len(containerID))], err)
		return fmt.Errorf("컨테이너 삭제 실패: %w", err)
	}

	log.Printf("[computer-use] 컨테이너 해제 완료: session=%s", sessionID)
	return nil
}

// StartReplenisher는 워밍 풀을 유지하는 백그라운드 고루틴을 시작한다.
// 컨텍스트가 취소되면 종료된다.
func (p *ContainerPool) StartReplenisher(ctx context.Context) {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			p.replenishWarmPool(ctx)
		}
	}
}

// replenishWarmPool은 워밍 풀이 설정 크기보다 작으면 컨테이너를 추가한다.
func (p *ContainerPool) replenishWarmPool(ctx context.Context) {
	p.mu.Lock()
	if p.shutdown {
		p.mu.Unlock()
		return
	}

	warmNeeded := p.config.WarmPoolSize - len(p.warmPool)
	totalCount := len(p.warmPool) + len(p.activePool)
	availableSlots := p.config.MaxContainers - totalCount

	// 생성 가능한 수 계산
	toCreate := warmNeeded
	if toCreate > availableSlots {
		toCreate = availableSlots
	}
	p.mu.Unlock()

	for i := 0; i < toCreate; i++ {
		if ctx.Err() != nil {
			return
		}

		info, err := p.manager.Create(ctx)
		if err != nil {
			log.Printf("[computer-use] 워밍 풀 보충 실패: %v", err)
			return
		}

		p.mu.Lock()
		if p.shutdown {
			p.mu.Unlock()
			_ = p.manager.Remove(ctx, info.ID)
			return
		}
		p.warmPool = append(p.warmPool, warmContainer{
			info:      info,
			createdAt: time.Now(),
		})
		log.Printf("[computer-use] 워밍 풀 보충: container=%s (warm=%d/%d)",
			info.ID[:min(12, len(info.ID))], len(p.warmPool), p.config.WarmPoolSize)
		p.mu.Unlock()
	}
}

// Shutdown은 모든 컨테이너를 정리하고 풀을 종료한다.
func (p *ContainerPool) Shutdown(ctx context.Context) error {
	p.mu.Lock()
	if p.shutdown {
		p.mu.Unlock()
		return nil
	}
	p.shutdown = true

	// 정리할 컨테이너 ID 수집
	var containerIDs []string
	for _, warm := range p.warmPool {
		containerIDs = append(containerIDs, warm.info.ID)
	}
	for _, active := range p.activePool {
		containerIDs = append(containerIDs, active.info.ID)
	}

	p.warmPool = nil
	p.activePool = make(map[string]activeContainer)
	p.mu.Unlock()

	// 컨테이너 정리 (잠금 없이)
	var errs []error
	for _, id := range containerIDs {
		if err := p.manager.Remove(ctx, id); err != nil {
			errs = append(errs, fmt.Errorf("컨테이너 %s 삭제 실패: %w", id[:min(12, len(id))], err))
		}
	}

	log.Printf("[computer-use] 컨테이너 풀 종료 완료: %d개 컨테이너 정리", len(containerIDs))

	if len(errs) > 0 {
		return fmt.Errorf("일부 컨테이너 정리 실패: %v", errs)
	}
	return nil
}

// Status는 풀의 현재 상태를 반환한다.
func (p *ContainerPool) Status() PoolStatus {
	p.mu.Lock()
	defer p.mu.Unlock()

	return PoolStatus{
		WarmCount:   len(p.warmPool),
		ActiveCount: len(p.activePool),
		MaxCount:    p.config.MaxContainers,
	}
}

// ActiveCount는 활성 컨테이너 수를 반환한다.
func (p *ContainerPool) ActiveCount() int {
	p.mu.Lock()
	defer p.mu.Unlock()
	return len(p.activePool)
}

// WarmCount는 워밍 풀의 컨테이너 수를 반환한다.
func (p *ContainerPool) WarmCount() int {
	p.mu.Lock()
	defer p.mu.Unlock()
	return len(p.warmPool)
}
