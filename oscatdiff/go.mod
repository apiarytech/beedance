module github.com/apiarytech/beedance/oscatdiff

go 1.27.3

require (
	github.com/apiarytech/beebread v0.2.2
	github.com/apiarytech/beedance v0.0.0-00010101000000-000000000000
)

require github.com/apiarytech/royaljelly v0.4.0 // indirect

// The tests are developed with beedance in this repository.
replace github.com/apiarytech/beedance => ..
