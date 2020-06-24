// +build linux
package main

func run() {
	generate([]string{
		"../engine/linux/libdeepspeech.so", "../engine/deepspeech_plugin.so"},
		"../libs_linux.go",
		"// +build linux",
	)
}
