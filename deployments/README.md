# Autopus Bridge 원격 Docker 배포 가이드

Autopus Local Bridge를 Docker 컨테이너로 원격 서버에 배포하기 위한 가이드입니다.

## 사전 요구사항

- Docker Engine 24.0 이상
- Docker Compose v2.20 이상
- Autopus 플랫폼 계정 및 JWT 인증 토큰
- (선택) SSH 키 쌍 (팀 원격 접속용)

## 빠른 시작

### 1. 환경 설정

```bash
cd deployments
cp .env.example .env
```

`.env` 파일을 편집하여 실제 값을 입력합니다:

```env
AUTOPUS_SERVER_URL=https://api.autopus.co
AUTOPUS_TOKEN=<발급받은-JWT-토큰>
AUTOPUS_WORKSPACE=<워크스페이스-ID>
```

### 2. 작업 디렉토리 생성

Bridge가 파일을 읽고 쓸 작업 디렉토리를 생성합니다:

```bash
mkdir -p workspace
```

### 3. 컨테이너 실행

```bash
docker compose up -d bridge
```

### 4. 상태 확인

```bash
docker compose ps
docker compose logs bridge
```

## SSH 터널 설정 (팀 원격 접속)

팀원이 원격 서버의 Bridge에 안전하게 접속할 수 있도록 SSH 터널 사이드카를 제공합니다.

### 1. SSH 키 준비

```bash
mkdir -p ssh-keys
```

접속을 허용할 팀원의 공개 키를 `ssh-keys/authorized_keys` 파일에 추가합니다:

```bash
cat ~/.ssh/id_rsa.pub >> ssh-keys/authorized_keys
```

### 2. SSH 사이드카 포함 실행

```bash
docker compose up -d
```

### 3. 클라이언트 측 SSH 터널 연결

팀원의 로컬 머신에서 다음 명령어를 실행합니다:

```bash
ssh -N -L 8765:bridge:8765 bridge@<서버-IP> -p 2222
```

이후 `localhost:8765`로 Bridge에 접속할 수 있습니다.

## 설정 옵션

| 환경변수 | 기본값 | 설명 |
|----------|--------|------|
| `AUTOPUS_SERVER_URL` | `https://api.autopus.co` | Autopus 서버 URL |
| `AUTOPUS_TOKEN` | (필수) | JWT 인증 토큰 |
| `AUTOPUS_WORKSPACE` | (필수) | 워크스페이스 ID |
| `SSH_USER` | `bridge` | SSH 접속 사용자명 |
| `SSH_PORT` | `2222` | SSH 포트 번호 |

## 볼륨 구성

| 볼륨 | 컨테이너 경로 | 설명 |
|------|---------------|------|
| `./workspace` | `/workspace` | AI 에이전트 작업 디렉토리 |
| `bridge-config` | `/root/.config/local-agent-bridge` | Bridge 내부 설정 (named volume) |
| `./ssh-keys` | `/config/authorized_keys` | SSH 공개 키 (읽기 전용) |

## 보안 고려사항

### 토큰 관리

- `.env` 파일은 절대 Git에 커밋하지 마세요. `.gitignore`에 반드시 추가하세요.
- 프로덕션 환경에서는 Docker Secrets 또는 외부 비밀 관리 시스템(Vault 등)을 사용하세요.
- JWT 토큰은 주기적으로 갱신하세요.

### 네트워크 격리

- Bridge 포트(`8765`)는 `127.0.0.1`에만 바인딩되어 외부에서 직접 접근할 수 없습니다.
- 외부 접속은 반드시 SSH 터널을 통해서만 허용하세요.
- `bridge-net` 네트워크는 컨테이너 간 내부 통신에만 사용됩니다.

### 파일 시스템

- `workspace` 디렉토리의 권한을 적절히 설정하세요 (`chmod 700 workspace`).
- Bridge 설정 파일은 `0600` 권한으로 저장됩니다.

## 문제 해결

### 컨테이너가 시작되지 않는 경우

```bash
# 로그 확인
docker compose logs bridge

# 환경변수 확인
docker compose config
```

### Health check 실패

```bash
# 컨테이너 내부에서 상태 확인
docker compose exec bridge autopus-bridge status

# 네트워크 연결 확인
docker compose exec bridge wget -q -O- https://api.autopus.co/health
```

### SSH 터널 연결 실패

```bash
# SSH 사이드카 로그 확인
docker compose logs ssh-tunnel

# SSH 키 권한 확인 (로컬)
chmod 600 ~/.ssh/id_rsa
chmod 644 ~/.ssh/id_rsa.pub

# authorized_keys 파일 확인
cat ssh-keys/authorized_keys
```

### 컨테이너 재시작

```bash
# 전체 재시작
docker compose restart

# Bridge만 재시작
docker compose restart bridge

# 클린 재시작 (볼륨 유지)
docker compose down && docker compose up -d
```

## 업데이트

새 버전의 Bridge 이미지가 배포되면 다음 명령어로 업데이트합니다:

```bash
docker compose pull bridge
docker compose up -d bridge
```
