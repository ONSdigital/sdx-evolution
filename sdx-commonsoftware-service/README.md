Common Software Service
======================

Adapter to the Common Software downstream (stub)

Currently logs receipt of message and then discards.

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
| DOWNSTREAM_EXCHANGE | `survey_downstream`                      | Name of rabbit exchange to publish to |
| LEGACY_EXCHANGE | `survey_legacy`                      | Name of rabbit exchange to get work from |
| NOTIFICATION_EXCHANGE | `survey_notify`                      | Name of rabbit exchange to bind `LEGACY_EXCHANGE` to |