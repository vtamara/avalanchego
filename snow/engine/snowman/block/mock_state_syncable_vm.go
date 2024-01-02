// Code generated by MockGen. DO NOT EDIT.
// Source: github.com/ava-labs/avalanchego/snow/engine/snowman/block (interfaces: StateSyncableVM)

// Package block is a generated GoMock package.
package block

import (
	context "context"
	reflect "reflect"

	ids "github.com/ava-labs/avalanchego/ids"
	gomock "go.uber.org/mock/gomock"
)

// MockStateSyncableVM is a mock of StateSyncableVM interface.
type MockStateSyncableVM struct {
	ctrl     *gomock.Controller
	recorder *MockStateSyncableVMMockRecorder
}

// MockStateSyncableVMMockRecorder is the mock recorder for MockStateSyncableVM.
type MockStateSyncableVMMockRecorder struct {
	mock *MockStateSyncableVM
}

// NewMockStateSyncableVM creates a new mock instance.
func NewMockStateSyncableVM(ctrl *gomock.Controller) *MockStateSyncableVM {
	mock := &MockStateSyncableVM{ctrl: ctrl}
	mock.recorder = &MockStateSyncableVMMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use.
func (m *MockStateSyncableVM) EXPECT() *MockStateSyncableVMMockRecorder {
	return m.recorder
}

// BackfillBlocks mocks base method.
func (m *MockStateSyncableVM) BackfillBlocks(arg0 context.Context, arg1 [][]byte) (ids.ID, uint64, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "BackfillBlocks", arg0, arg1)
	ret0, _ := ret[0].(ids.ID)
	ret1, _ := ret[1].(uint64)
	ret2, _ := ret[2].(error)
	return ret0, ret1, ret2
}

// BackfillBlocks indicates an expected call of BackfillBlocks.
func (mr *MockStateSyncableVMMockRecorder) BackfillBlocks(arg0, arg1 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "BackfillBlocks", reflect.TypeOf((*MockStateSyncableVM)(nil).BackfillBlocks), arg0, arg1)
}

// BackfillBlocksEnabled mocks base method.
func (m *MockStateSyncableVM) BackfillBlocksEnabled(arg0 context.Context) (ids.ID, uint64, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "BackfillBlocksEnabled", arg0)
	ret0, _ := ret[0].(ids.ID)
	ret1, _ := ret[1].(uint64)
	ret2, _ := ret[2].(error)
	return ret0, ret1, ret2
}

// BackfillBlocksEnabled indicates an expected call of BackfillBlocksEnabled.
func (mr *MockStateSyncableVMMockRecorder) BackfillBlocksEnabled(arg0 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "BackfillBlocksEnabled", reflect.TypeOf((*MockStateSyncableVM)(nil).BackfillBlocksEnabled), arg0)
}

// GetLastStateSummary mocks base method.
func (m *MockStateSyncableVM) GetLastStateSummary(arg0 context.Context) (StateSummary, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "GetLastStateSummary", arg0)
	ret0, _ := ret[0].(StateSummary)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// GetLastStateSummary indicates an expected call of GetLastStateSummary.
func (mr *MockStateSyncableVMMockRecorder) GetLastStateSummary(arg0 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "GetLastStateSummary", reflect.TypeOf((*MockStateSyncableVM)(nil).GetLastStateSummary), arg0)
}

// GetOngoingSyncStateSummary mocks base method.
func (m *MockStateSyncableVM) GetOngoingSyncStateSummary(arg0 context.Context) (StateSummary, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "GetOngoingSyncStateSummary", arg0)
	ret0, _ := ret[0].(StateSummary)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// GetOngoingSyncStateSummary indicates an expected call of GetOngoingSyncStateSummary.
func (mr *MockStateSyncableVMMockRecorder) GetOngoingSyncStateSummary(arg0 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "GetOngoingSyncStateSummary", reflect.TypeOf((*MockStateSyncableVM)(nil).GetOngoingSyncStateSummary), arg0)
}

// GetStateSummary mocks base method.
func (m *MockStateSyncableVM) GetStateSummary(arg0 context.Context, arg1 uint64) (StateSummary, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "GetStateSummary", arg0, arg1)
	ret0, _ := ret[0].(StateSummary)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// GetStateSummary indicates an expected call of GetStateSummary.
func (mr *MockStateSyncableVMMockRecorder) GetStateSummary(arg0, arg1 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "GetStateSummary", reflect.TypeOf((*MockStateSyncableVM)(nil).GetStateSummary), arg0, arg1)
}

// ParseStateSummary mocks base method.
func (m *MockStateSyncableVM) ParseStateSummary(arg0 context.Context, arg1 []byte) (StateSummary, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "ParseStateSummary", arg0, arg1)
	ret0, _ := ret[0].(StateSummary)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// ParseStateSummary indicates an expected call of ParseStateSummary.
func (mr *MockStateSyncableVMMockRecorder) ParseStateSummary(arg0, arg1 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "ParseStateSummary", reflect.TypeOf((*MockStateSyncableVM)(nil).ParseStateSummary), arg0, arg1)
}

// StateSyncEnabled mocks base method.
func (m *MockStateSyncableVM) StateSyncEnabled(arg0 context.Context) (bool, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "StateSyncEnabled", arg0)
	ret0, _ := ret[0].(bool)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// StateSyncEnabled indicates an expected call of StateSyncEnabled.
func (mr *MockStateSyncableVMMockRecorder) StateSyncEnabled(arg0 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "StateSyncEnabled", reflect.TypeOf((*MockStateSyncableVM)(nil).StateSyncEnabled), arg0)
}
