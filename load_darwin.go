// +build darwin

package deepspeech

func load() {
	extract("/engine/deepspeech_plugin.dylib", "/engine/mac/libdeepspeech.so")
}
