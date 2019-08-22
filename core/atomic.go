package core

import "sync/atomic"

type AtomicString struct {
	v atomic.Value
}

func NewAtomicString(str string) *AtomicString {
	s := &AtomicString{}
	if str != "" {
		s.Store(str)
	}
	return s
}

func (s *AtomicString) Load() string {
	v := s.v.Load()
	if v == nil {
		return ""
	}
	return v.(string)
}

func (s *AtomicString) Store(str string) {
	s.v.Store(str)
}

type AtomicBool int32

// Create a new atomic bool value, you should always use *AtomicBool in a struct to avoid copy
func NewAtomicBool(value bool) *AtomicBool {
	atomicBool := new(AtomicBool)
	atomicBool.Set(value)
	return atomicBool
}

func (b *AtomicBool) Get() bool {
	return atomic.LoadInt32((*int32)(b)) == 1
}

func (b *AtomicBool) Set(value bool) {
	if value {
		atomic.StoreInt32((*int32)(b), 1)
	} else {
		atomic.StoreInt32((*int32)(b), 0)
	}
}
