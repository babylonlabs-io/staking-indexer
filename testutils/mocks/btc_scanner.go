// Code generated by MockGen. DO NOT EDIT.
// Source: btcscanner/btc_scanner.go

// Package mocks is a generated GoMock package.
package mocks

import (
	reflect "reflect"

	btcscanner "github.com/babylonchain/staking-indexer/btcscanner"
	types "github.com/babylonchain/staking-indexer/types"
	gomock "github.com/golang/mock/gomock"
)

// MockBtcScanner is a mock of BtcScanner interface.
type MockBtcScanner struct {
	ctrl     *gomock.Controller
	recorder *MockBtcScannerMockRecorder
}

// MockBtcScannerMockRecorder is the mock recorder for MockBtcScanner.
type MockBtcScannerMockRecorder struct {
	mock *MockBtcScanner
}

// NewMockBtcScanner creates a new mock instance.
func NewMockBtcScanner(ctrl *gomock.Controller) *MockBtcScanner {
	mock := &MockBtcScanner{ctrl: ctrl}
	mock.recorder = &MockBtcScannerMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use.
func (m *MockBtcScanner) EXPECT() *MockBtcScannerMockRecorder {
	return m.recorder
}

// ChainUpdateInfoChan mocks base method.
func (m *MockBtcScanner) ChainUpdateInfoChan() <-chan *btcscanner.ChainUpdateInfo {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "ChainUpdateInfoChan")
	ret0, _ := ret[0].(<-chan *btcscanner.ChainUpdateInfo)
	return ret0
}

// ChainUpdateInfoChan indicates an expected call of ChainUpdateInfoChan.
func (mr *MockBtcScannerMockRecorder) ChainUpdateInfoChan() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "ChainUpdateInfoChan", reflect.TypeOf((*MockBtcScanner)(nil).ChainUpdateInfoChan))
}

// GetUnconfirmedBlocks mocks base method.
func (m *MockBtcScanner) GetUnconfirmedBlocks() ([]*types.IndexedBlock, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "GetUnconfirmedBlocks")
	ret0, _ := ret[0].([]*types.IndexedBlock)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// GetUnconfirmedBlocks indicates an expected call of GetUnconfirmedBlocks.
func (mr *MockBtcScannerMockRecorder) GetUnconfirmedBlocks() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "GetUnconfirmedBlocks", reflect.TypeOf((*MockBtcScanner)(nil).GetUnconfirmedBlocks))
}

// IsSynced mocks base method.
func (m *MockBtcScanner) IsSynced() bool {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "IsSynced")
	ret0, _ := ret[0].(bool)
	return ret0
}

// IsSynced indicates an expected call of IsSynced.
func (mr *MockBtcScannerMockRecorder) IsSynced() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "IsSynced", reflect.TypeOf((*MockBtcScanner)(nil).IsSynced))
}

// LastConfirmedHeight mocks base method.
func (m *MockBtcScanner) LastConfirmedHeight() uint64 {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "LastConfirmedHeight")
	ret0, _ := ret[0].(uint64)
	return ret0
}

// LastConfirmedHeight indicates an expected call of LastConfirmedHeight.
func (mr *MockBtcScannerMockRecorder) LastConfirmedHeight() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "LastConfirmedHeight", reflect.TypeOf((*MockBtcScanner)(nil).LastConfirmedHeight))
}

// Start mocks base method.
func (m *MockBtcScanner) Start(startHeight, activationHeight uint64) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Start", startHeight, activationHeight)
	ret0, _ := ret[0].(error)
	return ret0
}

// Start indicates an expected call of Start.
func (mr *MockBtcScannerMockRecorder) Start(startHeight, activationHeight interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Start", reflect.TypeOf((*MockBtcScanner)(nil).Start), startHeight, activationHeight)
}

// Stop mocks base method.
func (m *MockBtcScanner) Stop() error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Stop")
	ret0, _ := ret[0].(error)
	return ret0
}

// Stop indicates an expected call of Stop.
func (mr *MockBtcScannerMockRecorder) Stop() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Stop", reflect.TypeOf((*MockBtcScanner)(nil).Stop))
}
