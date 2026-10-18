# Third-party notices

This file records the production dependencies whose required notices were
verified for the current milestone-1 dependency baseline. Preserve these
attributions and license terms when redistributing the application. This is
not a complete inventory of transitive Go or frontend dependencies.

## React and React DOM

React and React DOM 19.3.1 are distributed under the MIT License.

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

## Remaining notice audit

The frontend dependency tree and complete Go module graph were not available in
the assigned documentation checkout, so the final production dependency set and
all required notices cannot yet be confirmed. The implementation owner must
inspect the resolved, distributable frontend and Go dependency graphs (including
transitives such as `scheduler` and `gopsutil`), copy every required copyright
and license notice from the actual versions, and update this document before
release. The reviewed plan identifies gopsutil's BSD-3-Clause-style license
and an upstream Go-derived notice; retain all required upstream terms. Do not
infer legal completeness from direct dependency manifests or this partial
inventory.
