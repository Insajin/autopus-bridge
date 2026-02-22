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

// replenishWarmPool은 유휴 워밍 컨테이너를 정리하고, 풀이 설정 크기보다 작으면 컨테이너를 추가한다.
func (p *ContainerPool) replenishWarmPool(ctx context.Context) {
	p.mu.Lock()
	if p.shutdown {
		p.mu.Unlock()
		return
	}

	// 유휴 워밍 컨테이너 정리 (REQ-C2-04)
	now := time.Now()
	var expiredContainers []string
	var remainingWarm []warmContainer
	for _, w := range p.warmPool {
		if now.Sub(w.createdAt) > p.config.IdleTimeout {
			expiredContainers = append(expiredContainers, w.info.ID)
		} else {
			remainingWarm = append(remainingWarm, w)
		}
	}
	p.warmPool = remainingWarm

	warmNeeded := p.config.WarmPoolSize - len(p.warmPool)
	totalCount := len(p.warmPool) + len(p.activePool)
	availableSlots := p.config.MaxContainers - totalCount

	// 생성 가능한 수 계산
	toCreate := warmNeeded
	if toCreate > availableSlots {
		toCreate = availableSlots
	}
	p.mu.Unlock()

	// 만료된 유휴 컨테이너 삭제 (잠금 해제 후 실행 - I/O 작업)
	for _, id := range expiredContainers {
		if err := p.manager.Remove(ctx, id); err != nil {
			log.Printf("[computer-use] 유휴 워밍 컨테이너 삭제 실패: container=%s, err=%v", id[:min(12, len(id))], err)
		} else {
			log.Printf("[computer-use] 유휴 워밍 컨테이너 삭제: container=%s", id[:min(12, len(id))])
		}
	}

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

// CleanupOrphaned는 시작 시 이전 실행에서 남은 orphaned 컨테이너를 정리한다.
// containerIDs는 정리할 컨테이너 ID 목록이다.
func (p *ContainerPool) CleanupOrphaned(ctx context.Context, containerIDs []string) error {
	if len(containerIDs) == 0 {
		return nil
	}

	log.Printf("[computer-use] orphaned 컨테이너 정리 시작: %d개", len(containerIDs))

	var errs []error
	for _, id := range containerIDs {
		if err := p.manager.Remove(ctx, id); err != nil {
			errs = append(errs, fmt.Errorf("orphaned 컨테이너 %s 삭제 실패: %w", id[:min(12, len(id))], err))
			log.Printf("[computer-use] orphaned 컨테이너 삭제 실패: container=%s, err=%v", id[:min(12, len(id))], err)
		} else {
			log.Printf("[computer-use] orphaned 컨테이너 삭제 완료: container=%s", id[:min(12, len(id))])
		}
	}

	if len(errs) > 0 {
		return fmt.Errorf("일부 orphaned 컨테이너 정리 실패: %v", errs)
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

// StartHealthMonitor는 활성 컨테이너의 health check를 주기적으로 수행하는 고루틴을 시작한다.
// 3회 연속 실패 시 컨테이너를 교체한다 (REQ-C1-07).
func (p *ContainerPool) StartHealthMonitor(ctx context.Context) {
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	// sessionID -> 연속 실패 횟수
	failCounts := make(map[string]int)

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			p.checkActiveContainerHealth(ctx, failCounts)
		}
	}
}

// checkActiveContainerHealth는 활성 컨테이너의 health를 확인하고 실패 시 교체한다.
func (p *ContainerPool) checkActiveContainerHealth(ctx context.Context, failCounts map[string]int) {
	p.mu.Lock()
	if p.shutdown {
		p.mu.Unlock()
		return
	}

	// 활성 컨테이너 목록 복사 (잠금 밖에서 health check 수행)
	type checkTarget struct {
		sessionID   string
		containerID string
		hostPort    string
	}
	var targets []checkTarget
	for sid, active := range p.activePool {
		targets = append(targets, checkTarget{
			sessionID:   sid,
			containerID: active.info.ID,
			hostPort:    active.info.HostPort,
		})
	}
	p.mu.Unlock()

	for _, target := range targets {
		err := p.manager.HealthCheck(ctx, target.containerID)
		if err != nil {
			failCounts[target.sessionID]++
			log.Printf("[computer-use] health check 실패 (%d/3): session=%s, container=%s, err=%v",
				failCounts[target.sessionID], target.sessionID, target.containerID[:min(12, len(target.containerID))], err)

			if failCounts[target.sessionID] >= 3 {
				log.Printf("[computer-use] 3회 연속 health check 실패, 컨테이너 교체: session=%s", target.sessionID)
				p.replaceUnhealthyContainer(ctx, target.sessionID)
				delete(failCounts, target.sessionID)
			}
		} else {
			// 성공 시 카운터 리셋
			delete(failCounts, target.sessionID)
		}
	}

	// 더 이상 활성이 아닌 세션의 카운터 정리
	p.mu.Lock()
	for sid := range failCounts {
		if _, exists := p.activePool[sid]; !exists {
			delete(failCounts, sid)
		}
	}
	p.mu.Unlock()
}

// replaceUnhealthyContainer는 비정상 컨테이너를 새 컨테이너로 교체한다.
func (p *ContainerPool) replaceUnhealthyContainer(ctx context.Context, sessionID string) {
	p.mu.Lock()
	active, exists := p.activePool[sessionID]
	if !exists {
		p.mu.Unlock()
		return
	}
	oldContainerID := active.info.ID
	p.mu.Unlock()

	// 새 컨테이너 생성
	newInfo, err := p.manager.Create(ctx)
	if err != nil {
		log.Printf("[computer-use] 교체 컨테이너 생성 실패: session=%s, err=%v", sessionID, err)
		return
	}

	p.mu.Lock()
	// 다시 확인 (락 해제 사이에 세션이 종료되었을 수 있음)
	if _, stillExists := p.activePool[sessionID]; !stillExists {
		p.mu.Unlock()
		_ = p.manager.Remove(ctx, newInfo.ID)
		return
	}

	p.activePool[sessionID] = activeContainer{
		info:       newInfo,
		sessionID:  sessionID,
		assignedAt: time.Now(),
	}
	p.mu.Unlock()

	// 이전 컨테이너 정리
	_ = p.manager.Remove(ctx, oldContainerID)

	log.Printf("[computer-use] 컨테이너 교체 완료: session=%s, old=%s, new=%s",
		sessionID, oldContainerID[:min(12, len(oldContainerID))], newInfo.ID[:min(12, len(newInfo.ID))])
}
