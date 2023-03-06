# openapi-directory

This loads APIs from
[apis-guru/openapi-directory](https://github.com/apis-guru/openapi-directory).

It makes some assumptions about API and version names that are based on the
directory structure.

- APIs are assumed to be in directories named for their owner and, if a second
  subdirectory exists, the name of the API.

- Below this, there is a directory with a version name. This import assumes
  that this is the version of the API, but it is usually the version of the
  OpenAPI description, which often differs. In some APIs (notably Twilio),
  version names are part of the name of the API directory name. This import
  leaves that inconsistency unresolved.

- OpenAPI 3.0 and later descriptions are in files named `openapi.yaml`. OpenAPI
  2.0 descriptions are in files named `swagger.yaml`.
