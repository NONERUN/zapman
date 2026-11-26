//go:build !windows

package service

import "errors"

// NewHost gives a Host that fails: the zapret service is only on Windows.
func NewHost() Host { return otherHost{} }

type otherHost struct{}

var errNotWindows = errors.New("the zapret service is only on Windows")

func (otherHost) Elevated() bool                              { return false }
func (otherHost) Query(string) (ServiceStatus, error)         { return ServiceStatus{}, errNotWindows }
func (otherHost) Create(_, _, _, _ string) error              { return errNotWindows }
func (otherHost) Delete(string) error                         { return errNotWindows }
func (otherHost) Start(string) error                          { return errNotWindows }
func (otherHost) Stop(string) error                           { return errNotWindows }
func (otherHost) ReadRecord(string) (Record, error)           { return Record{}, errNotWindows }
func (otherHost) WriteRecord(string, *Record) error           { return errNotWindows }
func (otherHost) KillEngines() (int, error)                   { return 0, errNotWindows }
