// Copyright (C) 2026 pi-apps-go contributors
// This file is part of Pi-Apps Go - a modern, cross-architecture/cross-platform, and modular Pi-Apps implementation in Go.
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU General Public License for more details.

// You should have received a copy of the GNU General Public License
// along with this program.  If not, see <https://www.gnu.org/licenses/>.

// Module: icu.go
// Description: Provides functions for getting the ICU version via the C API.
// SPDX-License-Identifier: GPL-3.0-or-later

package api

/*
#cgo LDFLAGS: -licuuc
#include <stdio.h>
#include <unicode/utypes.h>
#include <unicode/uversion.h>

void getICUVersion(char *buffer) {
    UVersionInfo versionArray;
    u_getVersion(versionArray);
    sprintf(buffer, "%d.%d.%d.%d", versionArray[0], versionArray[1], versionArray[2], versionArray[3]);
}
*/
import "C"
import "unsafe"

// Version returns the ICU version as a string, e.g. "73.1.0.0"
func GetICUVersion() string {
	var buffer [20]C.char
	C.getICUVersion((*C.char)(unsafe.Pointer(&buffer[0])))
	return C.GoString(&buffer[0])
}
