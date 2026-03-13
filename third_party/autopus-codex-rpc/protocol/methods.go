package protocol

// --- JSON-RPC 메서드 상수 (Bridge 스타일: MethodXxx 네이밍) ---

const (
	// MethodInitialize는 초기화 요청 메서드이다.
	MethodInitialize = "initialize"
	// MethodInitialized는 초기화 완료 알림 메서드이다.
	MethodInitialized = "initialized"

	// MethodAccountLoginStart는 인증 시작 메서드이다.
	MethodAccountLoginStart = "account/login/start"

	// MethodThreadStart는 Thread 생성 메서드이다.
	MethodThreadStart = "thread/start"
	// MethodThreadResume는 Thread 재개 메서드이다.
	MethodThreadResume = "thread/resume"

	// MethodTurnStart는 Turn 시작 메서드이다.
	MethodTurnStart = "turn/start"
	// MethodTurnInterrupt는 Turn 중단 메서드이다.
	MethodTurnInterrupt = "turn/interrupt"

	// MethodTurnCompleted는 Turn 완료 알림 메서드이다 (서버 -> 클라이언트).
	MethodTurnCompleted = "turn/completed"
	// MethodItemStarted는 Item 시작 알림 메서드이다 (서버 -> 클라이언트).
	MethodItemStarted = "item/started"
	// MethodItemCompleted는 Item 완료 알림 메서드이다 (서버 -> 클라이언트).
	MethodItemCompleted = "item/completed"

	// MethodAgentMessageDelta는 에이전트 메시지 증분 알림 메서드이다.
	MethodAgentMessageDelta = "item/agentMessage/delta"
	// MethodCommandExecutionOutputDelta는 명령 실행 출력 증분 알림 메서드이다.
	MethodCommandExecutionOutputDelta = "item/commandExecution/outputDelta"
	// MethodFileChangeOutputDelta는 파일 변경 출력 증분 알림 메서드이다.
	MethodFileChangeOutputDelta = "item/fileChange/outputDelta"

	// MethodCommandExecutionApproval은 명령 실행 승인 요청 알림 메서드이다.
	MethodCommandExecutionApproval = "item/commandExecution/requestApproval"
	// MethodFileChangeApproval은 파일 변경 승인 요청 알림 메서드이다.
	MethodFileChangeApproval = "item/fileChange/requestApproval"
)
