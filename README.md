# NAT-ALL-AWS-CONNECTOR
master : [![CircleCI](https://circleci.com/gh/ernestio/nat-all-aws-connector/tree/master.svg?style=svg)](https://circleci.com/gh/ernestio/nat-all-aws-connector/tree/master) | develop : [![CircleCI](https://circleci.com/gh/ernestio/nat-all-aws-connector/tree/develop.svg?style=svg)](https://circleci.com/gh/ernestio/nat-all-aws-connector/tree/develop)

Service to manage aws nats, it responds to:

- [x] nat.create.aws 
- [x] nat.update.aws 
- [x] nat.delete.aws 
- [ ] nat.get.aws 

And responds respectively with original_subject.error or original_subjet.done respectively

## Installation

```
make deps
make install
```

## Running Tests

```
make test
```

## Contributing

Please read through our
[contributing guidelines](CONTRIBUTING.md).
Included are directions for opening issues, coding standards, and notes on
development.

Moreover, if your pull request contains patches or features, you must include
relevant unit tests.

## Versioning

For transparency into our release cycle and in striving to maintain backward
compatibility, this project is maintained under [the Semantic Versioning guidelines](http://semver.org/).

## Copyright and License

Code and documentation copyright since 2015 r3labs.io authors.

Code released under
[the Mozilla Public License Version 2.0](LICENSE).

