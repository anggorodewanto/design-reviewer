# Phase 22: Zip Bomb — No Decompression Size Limit

**Severity:** 🔴 Critical

## Problem

The upload handler enforces a 50 MB limit on the compressed zip, but there is no limit on decompressed output. A crafted 50 MB zip can decompress to gigabytes, exhausting disk space. There is also no limit on the number of files extracted.

```go
// VULNERABLE — unbounded copy
_, err = io.Copy(out, rc)
```

## Fix

Add decompression size and file count limits:

```go
const maxDecompressedSize = 500 << 20 // 500 MB
const maxFileCount = 1000

// Before extraction loop:
if len(zr.File) > maxFileCount {
    return fmt.Errorf("zip contains too many files (max %d)", maxFileCount)
}

// Replace io.Copy with bounded copy:
var totalWritten int64
// ...per file:
n, err := io.Copy(out, io.LimitReader(rc, maxDecompressedSize-totalWritten))
totalWritten += n
if totalWritten > maxDecompressedSize {
    return fmt.Errorf("decompressed size exceeds limit")
}
```

## Files to Change

- `internal/storage/storage.go` — `SaveUpload` function

## Acceptance Criteria

- Zips with more than 1000 files are rejected
- Decompression aborts if total extracted size exceeds 500 MB
- Normal uploads within limits still work
