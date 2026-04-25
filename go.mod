module github.com/readsync/readsync

go 1.22

require (
	github.com/kardianos/service v1.2.2
	github.com/mattn/go-sqlite3 v1.14.22
)

// go-sqlite3 requires CGO. Build with CGO_ENABLED=1.
// On Windows: install TDM-GCC (https://jmeubank.github.io/tdm-gcc/)
// On Linux CI: sudo apt-get install gcc
//
// Phase 2 will add:
//   golang.org/x/net v0.24.0         (WebDAV server for Moon+)
//   golang.org/x/sys v0.19.0         (Windows registry access)
