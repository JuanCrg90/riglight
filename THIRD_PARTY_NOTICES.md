# Third-party notices

This file records the verified licenses for the current frontend runtime
packages. Preserve these attributions and license terms when redistributing the
application. The Go production dependency graph is not yet available, so this
is not the complete application notice inventory.

## React, React DOM, and scheduler

React 19.3.1, React DOM 19.3.1, and scheduler 0.28.0 are distributed under
the MIT License. `react-dom` depends on `scheduler`.

```text
MIT License

Copyright (c) Meta Platforms, Inc. and affiliates.

Permission is hereby granted, free of charge, to any person obtaining a copy
of this software and associated documentation files (the "Software"), to deal
in the Software without restriction, including without limitation the rights
to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
copies of the Software, and to permit persons to whom the Software is
furnished to do so, subject to the following conditions:

The above copyright notice and this permission notice shall be included in all
copies or substantial portions of the Software.

THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
SOFTWARE.
```

## Verified frontend sources

The package names and versions above were read from the installed package
manifests and their `LICENSE` files in the resolved frontend dependency tree.
The three packages carry identical MIT text reproduced above. `scheduler` is
a production transitive dependency of `react-dom`, not a development tool.

## gopsutil/v4 (declared Go dependency)

The backend `go.mod` declares `github.com/shirou/gopsutil/v4 v4.26.9` and
`go.sum` pins its module checksums. The module is currently marked indirect and
is not imported by backend source, so it is not yet in the compiled application
package graph. Its module metadata resolves these transitive module versions:

- `github.com/ebitengine/purego v0.11.1`
- `github.com/go-ole/go-ole v1.2.6`
- `github.com/google/go-cmp v0.7.0`
- `github.com/lufia/plan9stats v0.0.0-20211012122336-39d0f177ccd0`
- `github.com/power-devops/perfstat v0.0.0-20260805114148-88456608a4f6`
- `github.com/stretchr/testify v1.12.1`
- `github.com/tklauser/go-sysconf v0.4.0`
- `github.com/tklauser/numcpus v0.12.0`
- `github.com/yusufpapurcu/wmi v1.2.4`
- `go.yaml.in/yaml/v3 v3.0.5`
- `golang.org/x/sys v0.48.0`

The module graph includes test-only and platform-specific requirements; this
list is not an assertion that all are linked into the Linux binary. The Go
source/package graph must be rechecked after collector imports are added. In
particular, Linux CPU and memory files in gopsutil reference `go-sysconf` and
`x/sys`; other listed dependencies support other platforms or tests.

The upstream gopsutil v4.26.9 `LICENSE` was inspected from the pinned module
cache. Its full BSD license and embedded Go-derived notice are reproduced
below; retain both attributions and terms in distributions:

```text
gopsutil is distributed under BSD license reproduced below.

Copyright (c) 2014, WAKAYAMA Shirou
All rights reserved.

Redistribution and use in source and binary forms, with or without modification,
are permitted provided that the following conditions are met:

 * Redistributions of source code must retain the above copyright notice, this
   list of conditions and the following disclaimer.
 * Redistributions in binary form must reproduce the above copyright notice,
   this list of conditions and the following disclaimer in the documentation
   and/or other materials provided with the distribution.
 * Neither the name of the gopsutil authors nor the names of its contributors
   may be used to endorse or promote products derived from this software without
   specific prior written permission.

THIS SOFTWARE IS PROVIDED BY THE COPYRIGHT HOLDERS AND CONTRIBUTORS "AS IS" AND
ANY EXPRESS OR IMPLIED WARRANTIES, INCLUDING, BUT NOT LIMITED TO, THE IMPLIED
WARRANTIES OF MERCHANTABILITY AND FITNESS FOR A PARTICULAR PURPOSE ARE
DISCLAIMED. IN NO EVENT SHALL THE COPYRIGHT OWNER OR CONTRIBUTORS BE LIABLE FOR
ANY DIRECT, INDIRECT, INCIDENTAL, SPECIAL, EXEMPLARY, OR CONSEQUENTIAL DAMAGES
(INCLUDING, BUT NOT LIMITED TO, PROCUREMENT OF SUBSTITUTE GOODS OR SERVICES;
LOSS OF USE, DATA, OR PROFITS; OR BUSINESS INTERRUPTION) HOWEVER CAUSED AND ON
ANY THEORY OF LIABILITY, WHETHER IN CONTRACT, STRICT LIABILITY, OR TORT
(INCLUDING NEGLIGENCE OR OTHERWISE) ARISING IN ANY WAY OUT OF THE USE OF THIS
SOFTWARE, EVEN IF ADVISED OF THE POSSIBILITY OF SUCH DAMAGE.

-------
internal/common/binary.go in the gopsutil is copied and modified from golang/encoding/binary.go.

Copyright (c) 2009 The Go Authors. All rights reserved.

Redistribution and use in source and binary forms, with or without
modification, are permitted provided that the following conditions are
met:

   * Redistributions of source code must retain the above copyright
notice, this list of conditions and the following disclaimer.
   * Redistributions in binary form must reproduce the above
copyright notice, this list of conditions and the following disclaimer
in the documentation and/or other materials provided with the
       distribution.
   * Neither the name of Google Inc. nor the names of its
contributors may be used to endorse or promote products derived from
this software without specific prior written permission.

THIS SOFTWARE IS PROVIDED BY THE COPYRIGHT HOLDERS AND CONTRIBUTORS
"AS IS" AND ANY EXPRESS OR IMPLIED WARRANTIES, INCLUDING, BUT NOT
LIMITED TO, THE IMPLIED WARRANTIES OF MERCHANTABILITY AND FITNESS FOR
A PARTICULAR PURPOSE ARE DISCLAIMED. IN NO EVENT SHALL THE COPYRIGHT
OWNER OR CONTRIBUTORS BE LIABLE FOR ANY DIRECT, INDIRECT, INCIDENTAL,
SPECIAL, EXEMPLARY, OR CONSEQUENTIAL DAMAGES (INCLUDING, BUT NOT
LIMITED TO, PROCUREMENT OF SUBSTITUTE GOODS OR SERVICES; LOSS OF USE,
DATA, OR PROFITS; OR BUSINESS INTERRUPTION) HOWEVER CAUSED AND ON ANY
THEORY OF LIABILITY, WHETHER IN CONTRACT, STRICT LIABILITY, OR TORT
(INCLUDING NEGLIGENCE OR OTHERWISE) ARISING IN ANY WAY OUT OF THE USE
OF THIS SOFTWARE, EVEN IF ADVISED OF THE POSSIBILITY OF SUCH DAMAGE.
```

## Remaining Go notice audit

The module cache available for this read-only audit contained the gopsutil
source/license, but not extracted modules or license files for its external
requirements. Before release, once these modules are actually in the application
package graph, inspect the built-target dependency graph and each linked module's
license/copyright files, then add the notices required for those exact versions.
Do not assume every `go.mod` requirement is linked, or that this partial inventory
is complete for the final binary.
