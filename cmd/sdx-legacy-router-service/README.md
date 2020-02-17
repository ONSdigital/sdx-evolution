Legacy Router Service
======================

The "brains" of SDX. Performs configurable routing for received messages to
appropriate processing endpoints or dead lettering.

## API

Exposes the following endpoints:

| Endpoint          | Methods   | Description |
| ----------------- | --------- | ----------- |
| `/healthcheck`    | `GET`     | Standard healthcheck endpoint. Returns `200 OK` if service is up, along with a JSON doc descibing specific health |

## Environment

Expects the following environment to be set:

| Var                   | Example                              | Description                                              |
| --------------------- | ------------------------------------ | -------------------------------------------------------- |
| PORT                  | `"5000"`                             | String describing the port on which to start the service |
| RABBIT_URL            | `"amqp://rabbit:rabbit@rabbit:5762"` | Url on which to connect to a rabbitmq instance. Includes `amqp://` protocol |
| DOWNSTREAM_EXCHANGE | `survey_downstream`                      | Name of rabbit exchange to get work from |