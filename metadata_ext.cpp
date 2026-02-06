#include "exiv2/exiv2.hpp"
#include <string>
#include <cstring>

extern "C"  {
    char* getMetaData(const char* path) {
        try {
            Exiv2::Image::UniquePtr image = Exiv2::ImageFactory::open(path);
            image->readMetadata();
            Exiv2::ExifData &exifData = image->exifData();

            std::string dt = exifData["Exif.Image.DateTime"].toString();

            // Convert to a C-style string on the heap
            // strdup allocates memory that Go must later call C.free() on.
            return strdup(dt.c_str());
        } 
        catch(...) {
            return nullptr;
        }
    }
}