Survey Store Service
======================

Interface between SDX (and potentially others) and the ONS datastore (for the purposes
of this POC this a local database)

## API

Exposes the following endpoints:

| Endpoint          | Methods   | Description |
| ----------------- | --------- | ----------- |
| `/survey`         | `POST`    | Receiving point for survey data to be stored |
| `/healthcheck`    | `GET`     | Standard healthcheck endpoint. Returns `200 OK` if service is up, along with a JSON doc descibing specific health |

## Environment

Expects the following environment to be set:

| Var                   | Example                              | Description                                              |
| --------------------- | ------------------------------------ | -------------------------------------------------------- |
| PORT                  | `"5000"`                             | String describing the port on which to start the service |
