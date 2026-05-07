package errors

import (
	stderrs "errors"
	"fmt"
)

var (
	ErrInvalidJobID     = stderrs.New("invalid job id")
	ErrInvalidQueueName = stderrs.New("invalid queue name")
	ErrInvalidTimeout   = stderrs.New("invalid timeout")

	ErrRedisConnect = stderrs.New("redis connection error")
	ErrQueueEncode  = stderrs.New("queue encode error")
	ErrQueueEnqueue = stderrs.New("queue enqueue error")
	ErrQueueDequeue = stderrs.New("queue dequeue error")
	ErrQueueDecode  = stderrs.New("queue decode error")
	ErrQueueLen     = stderrs.New("queue length error")
	ErrQueueClose   = stderrs.New("queue close error")
)

// DomainError keeps a stable error kind and the root cause for inspection.
type DomainError struct {
	Kind error
	Op   string
	Err  error
}

func (e *DomainError) Error() string {
	if e == nil {
		return "<nil>"
	}

	if e.Op == "" && e.Err == nil {
		return e.Kind.Error()
	}

	if e.Op == "" {
		return fmt.Sprintf("%s: %v", e.Kind.Error(), e.Err)
	}

	if e.Err == nil {
		return fmt.Sprintf("%s (%s)", e.Kind.Error(), e.Op)
	}

	return fmt.Sprintf("%s (%s): %v", e.Kind.Error(), e.Op, e.Err)
}

func (e *DomainError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Err
}

func (e *DomainError) Is(target error) bool {
	if e == nil {
		return false
	}
	return target == e.Kind
}

func New(kind error, op string, err error) error {
	return &DomainError{Kind: kind, Op: op, Err: err}
}
