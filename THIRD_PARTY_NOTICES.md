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

## Remaining Go notice audit

The assigned backend checkout currently has only `module riglight` and the Go
version in `go.mod`; it has no `go.sum` or external module requirements. Thus
the final Go production dependency graph cannot yet be established. After the
backend adds its pinned gopsutil dependency, inspect the resolved Go module graph
and each shipped production dependency's license/copyright files, including
transitives; add their required notices here before release. The reviewed plan
identifies gopsutil/v4 v4.26.9 as BSD-3-Clause-style and notes a Go-derived
notice; preserve the actual upstream terms and attribution. Do not infer legal
completeness from planned versions or direct requirements.
