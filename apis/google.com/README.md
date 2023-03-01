# google.com

## Sources

- [Google API Discovery Service](https://developers.google.com/discovery)
- [Google API descriptions in the googleapis repository on GitHub](https://github.com/googleapis/googleapis)

## Notes

Both of the above sources are directly supported by Google teams. The APIs that
they list overlap but each source contains APIs not listed by the other. The
API Discovery Service lists APIs that are available as HTTP JSON services. The
googleapis repository lists APIs that are available both as gRPC services and
as HTTP JSON services that are automatically transcoded by Google's API
platform.

This collection includes API descriptions from both sources. Discovery
documents are stored as specs with the `discovery` ID and are JSON documents.
The googleapis descriptions are in the Protocol Buffers language. They are
stored as specs with the `protos` ID and are stored here as directories of
files that include all dependencies and are ready for compilation with
`protoc`, the Protocol Buffer compiler. These directories of files are bundled
into ZIP archives when they are imported to the registry.
