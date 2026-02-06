# Paths
EXIV2_SRC = ./build/_deps/exiv2-src
EXIV2_GENERATED = ./build

# Compiler flags - Adding -I$(EXIV2_GENERATED) specifically
CXXFLAGS = -std=c++17 -Wall -I$(EXIV2_SRC)/include -I$(EXIV2_GENERATED)
LDFLAGS = -L./build/lib -lexiv2 -lexpat -lz -lpthread

# Default target
all: build-go

# 1. Compile the C++ wrapper into an object file
metadata_ext.o: metadata_ext.cpp
	g++ $(CXXFLAGS) -c metadata_ext.cpp -o metadata_ext.o

# 2. Build the Go binary, linking the object file and Exiv2
build-go:
	CGO_ENABLED=1 \
	CGO_CXXFLAGS="$(CXXFLAGS)" \
	CGO_LDFLAGS="$(LDFLAGS)" \
	go build -o metadata_tool .

clean:
	rm -f *.o metadata_tool