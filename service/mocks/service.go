// Code generated by MockGen. DO NOT EDIT.
// Source: github.com/juju/juju/service (interfaces: Service)

// Package mocks is a generated GoMock package.
package mocks

import (
	reflect "reflect"

	gomock "github.com/golang/mock/gomock"
	common "github.com/juju/juju/service/common"
)

// MockService is a mock of Service interface.
type MockService struct {
	ctrl     *gomock.Controller
	recorder *MockServiceMockRecorder
}

// MockServiceMockRecorder is the mock recorder for MockService.
type MockServiceMockRecorder struct {
	mock *MockService
}

// NewMockService creates a new mock instance.
func NewMockService(ctrl *gomock.Controller) *MockService {
	mock := &MockService{ctrl: ctrl}
	mock.recorder = &MockServiceMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use.
func (m *MockService) EXPECT() *MockServiceMockRecorder {
	return m.recorder
}

// Conf mocks base method.
func (m *MockService) Conf() common.Conf {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Conf")
	ret0, _ := ret[0].(common.Conf)
	return ret0
}

// Conf indicates an expected call of Conf.
func (mr *MockServiceMockRecorder) Conf() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Conf", reflect.TypeOf((*MockService)(nil).Conf))
}

// Exists mocks base method.
func (m *MockService) Exists() (bool, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Exists")
	ret0, _ := ret[0].(bool)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// Exists indicates an expected call of Exists.
func (mr *MockServiceMockRecorder) Exists() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Exists", reflect.TypeOf((*MockService)(nil).Exists))
}

// Install mocks base method.
func (m *MockService) Install() error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Install")
	ret0, _ := ret[0].(error)
	return ret0
}

// Install indicates an expected call of Install.
func (mr *MockServiceMockRecorder) Install() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Install", reflect.TypeOf((*MockService)(nil).Install))
}

// InstallCommands mocks base method.
func (m *MockService) InstallCommands() ([]string, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "InstallCommands")
	ret0, _ := ret[0].([]string)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// InstallCommands indicates an expected call of InstallCommands.
func (mr *MockServiceMockRecorder) InstallCommands() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "InstallCommands", reflect.TypeOf((*MockService)(nil).InstallCommands))
}

// Installed mocks base method.
func (m *MockService) Installed() (bool, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Installed")
	ret0, _ := ret[0].(bool)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// Installed indicates an expected call of Installed.
func (mr *MockServiceMockRecorder) Installed() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Installed", reflect.TypeOf((*MockService)(nil).Installed))
}

// Name mocks base method.
func (m *MockService) Name() string {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Name")
	ret0, _ := ret[0].(string)
	return ret0
}

// Name indicates an expected call of Name.
func (mr *MockServiceMockRecorder) Name() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Name", reflect.TypeOf((*MockService)(nil).Name))
}

// Remove mocks base method.
func (m *MockService) Remove() error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Remove")
	ret0, _ := ret[0].(error)
	return ret0
}

// Remove indicates an expected call of Remove.
func (mr *MockServiceMockRecorder) Remove() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Remove", reflect.TypeOf((*MockService)(nil).Remove))
}

// Running mocks base method.
func (m *MockService) Running() (bool, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Running")
	ret0, _ := ret[0].(bool)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// Running indicates an expected call of Running.
func (mr *MockServiceMockRecorder) Running() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Running", reflect.TypeOf((*MockService)(nil).Running))
}

// Start mocks base method.
func (m *MockService) Start() error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Start")
	ret0, _ := ret[0].(error)
	return ret0
}

// Start indicates an expected call of Start.
func (mr *MockServiceMockRecorder) Start() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Start", reflect.TypeOf((*MockService)(nil).Start))
}

// StartCommands mocks base method.
func (m *MockService) StartCommands() ([]string, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "StartCommands")
	ret0, _ := ret[0].([]string)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// StartCommands indicates an expected call of StartCommands.
func (mr *MockServiceMockRecorder) StartCommands() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "StartCommands", reflect.TypeOf((*MockService)(nil).StartCommands))
}

// Stop mocks base method.
func (m *MockService) Stop() error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Stop")
	ret0, _ := ret[0].(error)
	return ret0
}

// Stop indicates an expected call of Stop.
func (mr *MockServiceMockRecorder) Stop() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Stop", reflect.TypeOf((*MockService)(nil).Stop))
}
