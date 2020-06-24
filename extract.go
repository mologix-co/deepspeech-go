package deepspeech

import (
	"bytes"
	"os"
)

func extract(plugin, ds string) {
	extractTo(plugin, "deepspeech_plugin.so")
	extractTo(ds, "libdeepspeech.so")
}

func extractTo(from, to string) {
	file, err := openEmbeddedFile(from)
	if err != nil {
		panic(err)
	}

	pluginFile, err := os.OpenFile(to, os.O_CREATE|os.O_TRUNC|os.O_RDWR, 0755)
	if err != nil {
		panic(err)
	}
	defer pluginFile.Close()
	reader := bytes.NewReader(file.data)
	reader.WriteTo(pluginFile)
}
