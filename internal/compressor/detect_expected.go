// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package compressor

import "bytes"

// plainExpectedDetectors maps formats whose detector is a pure boolean
// predicate with no per-format error flag (isError == false).
var plainExpectedDetectors = map[Format]func([]byte) bool{
	FormatNx:             isNx,
	FormatTurbo:          isTurbo,
	FormatBazel:          isBazel,
	FormatDockerPs:       isDockerPs,
	FormatDockerImages:   isDockerImages,
	FormatKubectlPods:    isKubectlGetPods,
	FormatNpmAudit:       isNpmAudit,
	FormatCypress:        isCypress,
	FormatCargoBuild:     isCargoBuild,
	FormatDotnetBuild:    isDotnetBuild,
	FormatEslint:         isEslint,
	FormatRuff:           isRuff,
	FormatRubocop:        isRubocop,
	FormatMypy:           isMypy,
	FormatGolangciLint:   isGolangciLint,
	FormatGoBuild:        isGoBuild,
	FormatGitStatus:      reGitStatusHead.Match,
	FormatGitStash:       isGitStashList,
	FormatMarkdown:       isGhMarkdownView,
	FormatGhPr:           isGhPrView,
	FormatGhChecks:       isGhPrChecks,
	FormatGhRunList:      isGhRunList,
	FormatGhRunLog:       isGhRunLog,
	FormatLogs:           isRepetitiveLog,
	FormatSearch:         reRipgrepMatch.Match,
	FormatCargoClippy:    isCargoClippy,
	FormatDockerBuild:    isDockerBuild,
	FormatDockerPush:     isDockerPush,
	FormatTerraformPlan:  isTerraformPlan,
	FormatNpmLs:          isNpmLs,
	FormatCargoTree:      isCargoTree,
	FormatGoModGraph:     isGoModGraph,
	FormatComposer:       isComposer,
	FormatPHPStan:        isPHPStan,
	FormatPsalm:          isPsalm,
	FormatArtisanMigrate: isArtisanMigrate,
	FormatArtisanRoutes:  isArtisanRouteList,
	FormatAzPipelines:    isAzPipelines,
}

// detectFormatExpected checks the caller's expected format detectors before
// falling back to the full detectFormat scan. High-precedence safety guards
// (go test -json, git log, unified diffs) still run first so quoted runner
// markers inside a stream, commit message, or diff are never misclassified.
func detectFormatExpected(input []byte, expected []Format) (Format, bool) {
	if len(expected) == 0 {
		return detectFormat(input)
	}
	if isGoJSON(input) {
		return FormatGoTest, bytes.Contains(input, []byte(`"Action":"fail"`))
	}
	if isGitLog(input) {
		return FormatGitLog, false
	}
	if isLockfileDiff(input) {
		return FormatLockfileDiff, false
	}
	if isGitShow(input) {
		return FormatGitShow, false
	}
	if isGitDiff(input) {
		return FormatGitDiff, false
	}
	if len(expected) > 1 {
		for _, f := range expected {
			if f == FormatJest {
				if format, isErr, ok := detectOneFormat(input, FormatJest); ok {
					return format, isErr
				}
				break
			}
		}
	}
	for _, f := range expected {
		if format, isErr, ok := detectOneFormat(input, f); ok {
			return format, isErr
		}
	}
	return detectFormat(input)
}

func detectGoTestFormat(input []byte) (Format, bool, bool) {
	if reGoVerboseRun.Match(input) {
		return FormatGoTest, reGoTestFail.Match(input), true
	}
	if reGoTestPass.Match(input) && bytes.HasPrefix(input, []byte("PASS\n")) &&
		!reJestSuites.Match(input) && !reJestTests.Match(input) && !reGoTestFail.Match(input) {
		return FormatGoTest, false, true
	}
	if isJest(input) {
		return FormatUnknown, false, false
	}
	if reGoTestFail.Match(input) {
		return FormatGoTest, true, true
	}
	if bytes.Contains(input, []byte("PASS\n")) || reGoTestPass.Match(input) || reGoTestNoTest.Match(input) {
		return FormatGoTest, false, true
	}
	return FormatUnknown, false, false
}

func detectOneFormat(input []byte, f Format) (Format, bool, bool) {
	if pred := plainExpectedDetectors[f]; pred != nil {
		if pred(input) {
			return f, false, true
		}
		return FormatUnknown, false, false
	}
	switch f {
	case FormatGoTest:
		return detectGoTestFormat(input)
	case FormatGitLog:
		if reGitLogCommit.Match(input) && !isGitShow(input) {
			return FormatGitLog, false, true
		}
	case FormatNextBuild:
		if isNextBuild(input) {
			return FormatNextBuild, reNextBuildError.Match(input), true
		}
	case FormatNpmInstall:
		if isNpmInstall(input) {
			format, isErr := detectNpmInstall(input)
			return format, isErr, true
		}
	case FormatPipInstall:
		if isPipInstall(input) {
			format, isErr := detectPipInstall(input)
			return format, isErr, true
		}
	case FormatPHPUnit:
		if isPHPUnit(input) {
			return FormatPHPUnit, rePHPUnitFailure.Match(input), true
		}
	case FormatFlutterTest:
		if isFlutterTest(input) {
			return FormatFlutterTest, reFlutterTestFailure.Match(input), true
		}
	case FormatMaven:
		if isMaven(input) {
			return FormatMaven, reMavenFailure.Match(input), true
		}
	case FormatGradle:
		if isGradle(input) {
			return FormatGradle, reGradleFailure.Match(input), true
		}
	case FormatJest:
		if isJest(input) {
			return FormatJest, reJestFail.Match(input) || reJestError.Match(input) || !(reJestSuites.Match(input) && reJestTests.Match(input)), true
		}
	case FormatPytest:
		if isPytest(input) {
			return FormatPytest, !isPlainPytestPass(input), true
		}
	case FormatVitest:
		if isVitest(input) {
			return FormatVitest, reVitestError.Match(input) || !reVitestTests.Match(input), true
		}
	case FormatPlaywright:
		if isPlaywright(input) {
			return FormatPlaywright, rePlaywrightError.Match(input) || !rePlaywrightSummary.Match(input), true
		}
	case FormatCargoTest:
		if isCargoTest(input) {
			return FormatCargoTest, reCargoTestError.Match(input) || !cargoTestLinesRecognised(input) || !reCargoTestSummary.Match(input), true
		}
	case FormatTSC:
		if isTSC(input) {
			return FormatTSC, reTSCError.Match(input) || !reTSCPass.Match(input), true
		}
	case FormatRspec:
		if isRspec(input) {
			return FormatRspec, reRspecFailure.Match(input), true
		}
	case FormatDotnetTest:
		if isDotnetTest(input) {
			return FormatDotnetTest, reDotnetTestFailure.Match(input), true
		}
	case FormatFileList:
		if bytes.Contains(input, []byte("total ")) && bytes.Contains(input, []byte("drwx")) {
			return FormatFileList, false, true
		}
	}
	return FormatUnknown, false, false
}
