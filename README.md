SDX 2 - Onyx Gazelle
====================

Proof of concept for SDX 2 (codename Onyx Gazelle)

> _(!) This is pure poc and does not contain production ready code!_

## Get Started

```shell
$ make
$ make docker
```

 - `make` - build binaries for each service
 - `make docker` - run `docker-compose up --build` to create images based on binaries built with `make`

> nb. It may be advantageous if you make changes to queues to remove any existing rabbitmq container used by this docker-compose.

## Further evolution

Other points that aren't addressed yet but need to be:

 - Graceful shutdown (SIGTERM awareness) for services

## Notes

### Verboseness

The inline commenting of the services here is intentionally excessive to explain
decisions and operation to one not so familiar with the new services and Go
in general.

### Completeness

This POC is not designed in _any way to be production ready_ and has (likely) many
bugs/security issues (though some attempt at developing best practice is made).

### Local Dependencies

Local dependenices (libs written to support this POC) can be found in `/lib`.
This is done just to keep all the code nicely in a single repo that in itself
is pretending to be multiple repos.

> _THIS IS NOT THE CORRECT WAY TO DO IT BUT IT'S MINIMAL LIFTING FOR THE POC_

## LICENCE

Copyright (c) 2016 Crown Copyright (Office for National Statistics)

Released under MIT license, see [LICENSE](LICENSE) for details.