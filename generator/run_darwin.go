// +build darwin
package main

func run() {
	generate([]string{
		"../engine/mac/libdeepspeech.so", "../engine/deepspeech_plugin.dylib"},
		"../libs_darwin.go",
		"// +build darwin",
	)
}
