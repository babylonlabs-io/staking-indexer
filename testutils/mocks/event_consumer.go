// Code generated by MockGen. DO NOT EDIT.
// Source: consumer/event_consumer.go

// Package mocks is a generated GoMock package.
package mocks

import (
	reflect "reflect"

	client "github.com/babylonchain/staking-queue-client/client"
	gomock "github.com/golang/mock/gomock"
)

// MockEventConsumer is a mock of EventConsumer interface.
type MockEventConsumer struct {
	ctrl     *gomock.Controller
	recorder *MockEventConsumerMockRecorder
}

// MockEventConsumerMockRecorder is the mock recorder for MockEventConsumer.
type MockEventConsumerMockRecorder struct {
	mock *MockEventConsumer
}

// NewMockEventConsumer creates a new mock instance.
func NewMockEventConsumer(ctrl *gomock.Controller) *MockEventConsumer {
	mock := &MockEventConsumer{ctrl: ctrl}
	mock.recorder = &MockEventConsumerMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use.
func (m *MockEventConsumer) EXPECT() *MockEventConsumerMockRecorder {
	return m.recorder
}

// PushStakingEvent mocks base method.
func (m *MockEventConsumer) PushStakingEvent(ev *client.ActiveStakingEvent) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "PushStakingEvent", ev)
	ret0, _ := ret[0].(error)
	return ret0
}

// PushStakingEvent indicates an expected call of PushStakingEvent.
func (mr *MockEventConsumerMockRecorder) PushStakingEvent(ev interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "PushStakingEvent", reflect.TypeOf((*MockEventConsumer)(nil).PushStakingEvent), ev)
}

// PushUnbondingEvent mocks base method.
func (m *MockEventConsumer) PushUnbondingEvent(ev *client.UnbondingStakingEvent) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "PushUnbondingEvent", ev)
	ret0, _ := ret[0].(error)
	return ret0
}

// PushUnbondingEvent indicates an expected call of PushUnbondingEvent.
func (mr *MockEventConsumerMockRecorder) PushUnbondingEvent(ev interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "PushUnbondingEvent", reflect.TypeOf((*MockEventConsumer)(nil).PushUnbondingEvent), ev)
}

// PushUnconfirmedInfoEvent mocks base method.
func (m *MockEventConsumer) PushUnconfirmedInfoEvent(ev *client.UnconfirmedInfoEvent) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "PushUnconfirmedInfoEvent", ev)
	ret0, _ := ret[0].(error)
	return ret0
}

// PushUnconfirmedInfoEvent indicates an expected call of PushUnconfirmedInfoEvent.
func (mr *MockEventConsumerMockRecorder) PushUnconfirmedInfoEvent(ev interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "PushUnconfirmedInfoEvent", reflect.TypeOf((*MockEventConsumer)(nil).PushUnconfirmedInfoEvent), ev)
}

// PushWithdrawEvent mocks base method.
func (m *MockEventConsumer) PushWithdrawEvent(ev *client.WithdrawStakingEvent) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "PushWithdrawEvent", ev)
	ret0, _ := ret[0].(error)
	return ret0
}

// PushWithdrawEvent indicates an expected call of PushWithdrawEvent.
func (mr *MockEventConsumerMockRecorder) PushWithdrawEvent(ev interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "PushWithdrawEvent", reflect.TypeOf((*MockEventConsumer)(nil).PushWithdrawEvent), ev)
}

// Start mocks base method.
func (m *MockEventConsumer) Start() error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Start")
	ret0, _ := ret[0].(error)
	return ret0
}

// Start indicates an expected call of Start.
func (mr *MockEventConsumerMockRecorder) Start() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Start", reflect.TypeOf((*MockEventConsumer)(nil).Start))
}

// Stop mocks base method.
func (m *MockEventConsumer) Stop() error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Stop")
	ret0, _ := ret[0].(error)
	return ret0
}

// Stop indicates an expected call of Stop.
func (mr *MockEventConsumerMockRecorder) Stop() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Stop", reflect.TypeOf((*MockEventConsumer)(nil).Stop))
}
