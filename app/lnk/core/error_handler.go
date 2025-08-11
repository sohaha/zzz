package core

import (
	"fmt"
	"strings"
)

type ErrorCode string

const (
	ErrCodeRepoNotInitialized ErrorCode = "REPO_NOT_INITIALIZED"
	ErrCodeRepoAlreadyExists  ErrorCode = "REPO_ALREADY_EXISTS"
	ErrCodeRepoInvalid        ErrorCode = "REPO_INVALID"

	ErrCodeFileNotExists      ErrorCode = "FILE_NOT_EXISTS"
	ErrCodeFileAlreadyManaged ErrorCode = "FILE_ALREADY_MANAGED"
	ErrCodeFileNotManaged     ErrorCode = "FILE_NOT_MANAGED"
	ErrCodeFilePermission     ErrorCode = "FILE_PERMISSION"
	ErrCodeFileOperation      ErrorCode = "FILE_OPERATION"

	ErrCodeGitCommand       ErrorCode = "GIT_COMMAND"
	ErrCodeGitNetwork       ErrorCode = "GIT_NETWORK"
	ErrCodeGitAuth          ErrorCode = "GIT_AUTH"
	ErrCodeGitMergeConflict ErrorCode = "GIT_MERGE_CONFLICT"

	ErrCodeHostNotFound ErrorCode = "HOST_NOT_FOUND"

	ErrCodeBootstrapNotFound  ErrorCode = "BOOTSTRAP_NOT_FOUND"
	ErrCodeBootstrapExecution ErrorCode = "BOOTSTRAP_EXECUTION"
)

type ErrorSeverity string

const (
	SeverityInfo     ErrorSeverity = "INFO"
	SeverityWarning  ErrorSeverity = "WARNING"
	SeverityError    ErrorSeverity = "ERROR"
	SeverityCritical ErrorSeverity = "CRITICAL"
)

type StructuredError interface {
	error
	Code() ErrorCode
	Severity() ErrorSeverity
	Context() map[string]interface{}
	Suggestion() string
	Recoverable() bool
}

type BaseStructuredError struct {
	code        ErrorCode
	message     string
	severity    ErrorSeverity
	context     map[string]interface{}
	suggestion  string
	recoverable bool
	cause       error
}

func NewStructuredError(code ErrorCode, message string, severity ErrorSeverity) *BaseStructuredError {
	return &BaseStructuredError{
		code:        code,
		message:     message,
		severity:    severity,
		context:     make(map[string]interface{}),
		recoverable: false,
	}
}

func (e *BaseStructuredError) Error() string {
	var parts []string
	parts = append(parts, fmt.Sprintf("[%s]", e.code))
	parts = append(parts, e.message)

	if len(e.context) > 0 {
		var contextParts []string
		for k, v := range e.context {
			contextParts = append(contextParts, fmt.Sprintf("%s=%v", k, v))
		}
		parts = append(parts, fmt.Sprintf("(%s)", strings.Join(contextParts, ", ")))
	}

	if e.cause != nil {
		parts = append(parts, fmt.Sprintf("原因: %v", e.cause))
	}

	return strings.Join(parts, " ")
}

func (e *BaseStructuredError) Code() ErrorCode {
	return e.code
}

func (e *BaseStructuredError) Severity() ErrorSeverity {
	return e.severity
}

func (e *BaseStructuredError) Context() map[string]interface{} {
	return e.context
}

func (e *BaseStructuredError) Suggestion() string {
	return e.suggestion
}

func (e *BaseStructuredError) Recoverable() bool {
	return e.recoverable
}

func (e *BaseStructuredError) Unwrap() error {
	return e.cause
}

func (e *BaseStructuredError) WithContext(key string, value interface{}) *BaseStructuredError {
	e.context[key] = value
	return e
}

func (e *BaseStructuredError) WithSuggestion(suggestion string) *BaseStructuredError {
	e.suggestion = suggestion
	return e
}

func (e *BaseStructuredError) WithRecoverable(recoverable bool) *BaseStructuredError {
	e.recoverable = recoverable
	return e
}

func (e *BaseStructuredError) WithCause(cause error) *BaseStructuredError {
	e.cause = cause
	return e
}

type RollbackAction func() error

type RollbackManager struct {
	actions []RollbackAction
	enabled bool
}

func NewRollbackManager() *RollbackManager {
	return &RollbackManager{
		actions: make([]RollbackAction, 0),
		enabled: true,
	}
}

func (rm *RollbackManager) AddAction(action RollbackAction) {
	if rm.enabled {
		rm.actions = append(rm.actions, action)
	}
}

func (rm *RollbackManager) Execute() []error {
	var errors []error

	for i := len(rm.actions) - 1; i >= 0; i-- {
		if err := rm.actions[i](); err != nil {
			errors = append(errors, fmt.Errorf("回滚操作失败: %w", err))
		}
	}

	return errors
}

func (rm *RollbackManager) Clear() {
	rm.actions = rm.actions[:0]
}

func (rm *RollbackManager) Disable() {
	rm.enabled = false
}

func (rm *RollbackManager) Enable() {
	rm.enabled = true
}

type ErrorHandler struct {
	rollbackManager *RollbackManager
}

func NewErrorHandler() *ErrorHandler {
	return &ErrorHandler{
		rollbackManager: NewRollbackManager(),
	}
}

func (eh *ErrorHandler) HandleError(err error) error {
	if err == nil {
		return nil
	}

	if structErr, ok := err.(StructuredError); ok {
		if structErr.Recoverable() {
			rollbackErrors := eh.rollbackManager.Execute()
			if len(rollbackErrors) > 0 {

				return NewStructuredError(
					ErrCodeFileOperation,
					fmt.Sprintf("操作失败且回滚时发生错误: %v", err),
					SeverityCritical,
				).WithContext("rollback_errors", rollbackErrors)
			}
		}
	}

	return err
}

func (eh *ErrorHandler) AddRollbackAction(action RollbackAction) {
	eh.rollbackManager.AddAction(action)
}

func (eh *ErrorHandler) ClearRollback() {
	eh.rollbackManager.Clear()
}

func WrapError(err error, code ErrorCode, message string, severity ErrorSeverity) *BaseStructuredError {
	return NewStructuredError(code, message, severity).WithCause(err)
}

func IsErrorCode(err error, code ErrorCode) bool {
	if structErr, ok := err.(StructuredError); ok {
		return structErr.Code() == code
	}
	return false
}

func GetErrorSeverity(err error) ErrorSeverity {
	if structErr, ok := err.(StructuredError); ok {
		return structErr.Severity()
	}
	return SeverityError
}

func FormatErrorWithSuggestion(err error) string {
	if structErr, ok := err.(StructuredError); ok {
		message := err.Error()
		if suggestion := structErr.Suggestion(); suggestion != "" {
			message += fmt.Sprintf("\n建议: %s", suggestion)
		}
		return message
	}
	return err.Error()
}
