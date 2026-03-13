// Package client는 JSON-RPC 2.0 stdio 기반 클라이언트를 제공한다.
// 외부 의존성 없이 stdlib만 사용하며, Logger 인터페이스로 로깅을 추상화한다.
package client

// Logger는 클라이언트 로깅 인터페이스이다.
// 외부 의존성 없이 다양한 로거를 주입할 수 있다.
type Logger interface {
	// Debug는 디버그 레벨 로그를 기록한다.
	Debug(msg string, keysAndValues ...interface{})
	// Warn은 경고 레벨 로그를 기록한다.
	Warn(msg string, keysAndValues ...interface{})
	// Error는 에러 레벨 로그를 기록한다.
	Error(msg string, keysAndValues ...interface{})
}

// nopLogger는 로그를 버리는 no-op Logger 구현체이다.
type nopLogger struct{}

func (nopLogger) Debug(_ string, _ ...interface{}) {}
func (nopLogger) Warn(_ string, _ ...interface{})  {}
func (nopLogger) Error(_ string, _ ...interface{}) {}

// NopLogger는 로그를 버리는 Logger를 반환한다.
// 테스트 또는 로깅이 필요 없는 환경에서 사용한다.
func NopLogger() Logger {
	return nopLogger{}
}
