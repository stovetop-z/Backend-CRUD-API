cmake_minimum_required(VERSION 3.14)

project(metadata_ext)

include(FetchContent)

FetchContent_Declare(
    exiv2
    GIT_REPOSITORY https://github.com/Exiv2/exiv2.git
    GIT_TAG    v0.28.3
)

# Optimization: Tells Exiv2 NOT to build its own demo programs
set(EXIV2_BUILD_SAMPLES OFF CACHE BOOL "" FORCE)
set(EXIV2_BUILD_UNIT_TESTS OFF CACHE BOOL "" FORCE)

FetchContent_MakeAvailable(exiv2)

add_library(metadata_ext STATIC metadata_ext.cpp)

target_link_libraries(metadata_ext PRIVATE exiv2lib)