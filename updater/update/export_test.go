// SILVER - Service Wrapper
// Auto Updater
//
// Copyright (c) 2026 PaperCut Software http://www.papercut.com/
// Use of this source code is governed by an MIT or GPL Version 2 license.
// See the project's LICENSE file for more information.
//

package update

// Test-only exports for black-box tests in package update_test.
var (
	SetOSHeaders         = setOSHeaders
	TrimToNumericVersion = trimToNumericVersion
	UserAgent            = userAgent
)
