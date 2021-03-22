package errutil

import (
	"errors"
)

type Causer interface {
	Cause() error
}

type Wrapper interface {
	Unwrap() error
}

func Cause(err error) error {
	for e := Unwrap(err); e != nil; e = errors.Unwrap(e) {
		err = e
	}
	return err
}

func Unwrap(err error) error {
	switch e := err.(type) {
	case Causer:
		err = e.Cause()
	case Wrapper:
		err = e.Unwrap()
	default:
		err = nil
	}
	return err
}
