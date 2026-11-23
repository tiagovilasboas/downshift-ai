// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package core

import (
	"time"

	"github.com/tiagovilasboas/downshift-ai/internal/quotamodel"
	"github.com/tiagovilasboas/downshift-ai/internal/session"
)

// QuotaHooks holds the quota-package callbacks installed by RegisterQuota.
type QuotaHooks struct {
	Load           func(harness string) (*quotamodel.Snapshot, bool)
	LoadNative     func(harness string) (*quotamodel.Snapshot, bool)
	ParseNative    func(source string, data []byte, observed time.Time, ttl time.Duration) (quotamodel.Snapshot, error)
	RegisterNative func(harness, env, source string)
}

// RegisterQuota installs h onto core and mirrors the snapshot readers onto
// session, then publishes each harness's native quota source. The returned
// function restores the previous hook values.
func RegisterQuota(h QuotaHooks) (restore func()) {
	prevLoad := LoadQuotaHook
	prevLoadNative := LoadNativeQuotaHook
	prevParseNative := ParseNativeQuotaHook
	prevRegisterNative := RegisterNativeQuotaHook
	prevSessionLoad := session.LoadQuotaHook
	prevSessionLoadNative := session.LoadNativeQuotaHook

	LoadQuotaHook = h.Load
	LoadNativeQuotaHook = h.LoadNative
	ParseNativeQuotaHook = h.ParseNative
	RegisterNativeQuotaHook = h.RegisterNative
	session.LoadQuotaHook = h.Load
	session.LoadNativeQuotaHook = h.LoadNative
	PublishNativeQuotaSources()

	return func() {
		LoadQuotaHook = prevLoad
		LoadNativeQuotaHook = prevLoadNative
		ParseNativeQuotaHook = prevParseNative
		RegisterNativeQuotaHook = prevRegisterNative
		session.LoadQuotaHook = prevSessionLoad
		session.LoadNativeQuotaHook = prevSessionLoadNative
	}
}

// LoadQuotaHook reads the canonical quota cache for one harness.
// quota registers it at init so core does not import the filesystem package.
// A nil hook is missing evidence.
var LoadQuotaHook func(harness string) (*quotamodel.Snapshot, bool)

// LoadNativeQuotaHook reads an opt-in native export before the cache.
// quota registers it at init. A nil hook is missing evidence.
var LoadNativeQuotaHook func(harness string) (*quotamodel.Snapshot, bool)

// ParseNativeQuotaHook decodes a native usage payload into a snapshot.
// quota registers it at init. A nil hook fails closed.
var ParseNativeQuotaHook func(source string, data []byte, observed time.Time, ttl time.Duration) (quotamodel.Snapshot, error)

// RegisterNativeQuotaHook records a harness's native export env and source.
// quota registers it at init. capabilities publishes the table through it.
var RegisterNativeQuotaHook func(harness, env, source string)

// PublishNativeQuotaSources hands each harness's native export to the
// registered quota hook. quota calls this from init after assigning
// RegisterNativeQuotaHook, because this package's init runs first.
func PublishNativeQuotaSources() {
	if RegisterNativeQuotaHook == nil {
		return
	}
	for _, n := range AllHarnessCaps() {
		RegisterNativeQuotaHook(n.Harness, n.Caps.NativeSessionEnv, n.Caps.NativeUsageSource)
	}
}
