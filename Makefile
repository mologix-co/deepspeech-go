UNAME_S := $(shell uname -s)
UNAME_M := $(shell uname -m)
PLUGIN_NAME=

ifeq ($(UNAME_S),Linux)
	PLUGIN_NAME=deepspeech_plugin.so
endif
ifeq ($(UNAME_S),Darwin)
	PLUGIN_NAME=deepspeech_plugin.dylib
endif

build: build-plugin generate-static

build-plugin:
	cd engine; go build -buildmode=plugin -ldflags="-s -w" -o $(PLUGIN_NAME)

generate-static:
	cd generator; go build; ./generator
