// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package contextopt

// CCRSamplesEnabled resolves the raw-sample switch for output no compressor
// handled. env is DOWNSHIFT_CCR_SAMPLES ("1" on, "0" off, wins over the file);
// ccrEnv is DOWNSHIFT_CCR, whose "0" turns it off. Otherwise it is on unless
// the file says "ccr_samples": false.
func CCRSamplesEnabled(cfg Config, cfgErr error, env, ccrEnv string) bool {
	switch env {
	case "1":
		return true
	case "0":
		return false
	}
	if ccrEnv == "0" {
		return false
	}
	if cfgErr != nil || cfg.CCRSamples == nil {
		return true
	}
	return *cfg.CCRSamples
}
