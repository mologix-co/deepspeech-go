// +build linux

package deepspeech

func load() {
	extract("/engine/deepspeech_plugin.so", "/engine/linux/libdeepspeech.so")
}
