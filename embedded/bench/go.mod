module github.com/apiarytech/beedance/embedded/bench

go 1.27.2

require (
	github.com/apiarytech/beedance v0.0.0
	github.com/apiarytech/royaljelly v0.3.3
)

require golang.org/x/sys v0.47.0 // indirect

// The benchmark is developed with beedance in this repository.
replace github.com/apiarytech/beedance => ../..
