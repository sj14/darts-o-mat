module github.com/sj14/darts-o-mat/tools/winpkg

go 1.27.2

// The icon renderer and the version (tools/internal) come from the project itself.
replace github.com/sj14/darts-o-mat => ../..

require (
	github.com/sj14/darts-o-mat v0.0.0-00010101000000-000000000000
	github.com/tc-hib/winres v0.3.1
)

require (
	github.com/nfnt/resize v0.0.0-20180221191011-83c6a9932646 // indirect
	golang.org/x/image v0.12.0 // indirect
)
