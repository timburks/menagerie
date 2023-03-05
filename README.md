# Menagerie of APIs

This is a collection of API descriptions from a variety of sources and in a
variety of formats.

Coverage is very uneven! When we see something that might be interesting, we
create a directory for it in `apis` with a `README.md` that points to related
information. Over time, we hope to automatically digest more and more of these
APIs into
[Registry YAML](https://github.com/apigee/registry/wiki/Registry-YAML) to allow
them to be easily imported into a Registry instance using
[registry apply](https://github.com/apigee/registry/wiki/registry-apply).

## Ownership of API descriptions

All API descriptions are assumed copyrighted by their authors; typically these
are the organizations that own the domains under which they are stored. If any
listed APIs should not be included, please contact us and they will immediately
be removed.

## Guiding ideas

In all cases, we seek to import API descriptions in their original forms.
Instead of inserting metadata into specs, we add it in
[Registry YAML](https://github.com/apigee/registry/wiki/Registry-YAML). This
means we also preserve the original formats. Conversion to normalized formats
would be done by tools that run on a registry service. For an example, see
[registy-experimental generate openapi](https://github.com/apigee/registry-experimental/blob/main/cmd/registry-experimental/cmd/generate/openapi.go).

We also want to make this collection maintainable by automating imports as much
as possible.

## Getting this into a registry

- Get set up with the [Registry API](https://github.com/apigee/registry).
- Run `source tools/CREATE.sh` to create the "menagerie" project and configure
  the `registry` tool to use it. If you're using a Google-hosted instance of
  the Registry API, you can point the `registry` tool at it with
  `registry config set registry.project $PROJECTID`.
- Run `registry apply -f menagerie -R` to load the menagerie project. As the
  number of APIs grows, you might want to load specific subdirectories. For
  example, to just import the Google APIs, run
  `registry apply -f menagerie/apis/google.com -R`.

## The importers

Tooling for automated imports is in the `tools` directory. Expect this to be
unstable and likely to change, but if you have APIs that you want to import
into a registry, these importers can provide helpful patterns.

## License

This software is licensed under the Apache License, Version 2.0. See
[LICENSE](LICENSE) for the full license text.

## Disclaimer

This is not an official Google product. Issues filed on GitHub are not subject
to service level agreements (SLAs) and responses should be assumed to be on an
ad-hoc volunteer basis.

## Contributing

Contributions are welcome! Please see [CONTRIBUTING](CONTRIBUTING.md) for notes
on how to contribute to this project.
