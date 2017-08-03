Survey Gateway Service
======================

Serves as the entry point to SDX as the gateway into ONS.

## API

Exposes the following endpoints:

| Endpoint          | Methods   | Description |
| ----------------- | --------- | ----------- |
| `/survey`         | `POST`    | Receiving point for encrypted survey data |
| `/healthcheck`    | `GET`     | Standard healthcheck endpoint. Returns `200 OK` if service is up, along with a JSON doc descibing specific health |

## Environment

Expects the following environment to be set:

| Var                   | Example                              | Description                                              |
| --------------------- | ------------------------------------ | -------------------------------------------------------- |
| PORT                  | `"5000"`                             | String describing the port on which to start the service |
| RABBIT_URL            | `"amqp://rabbit:rabbit@rabbit:5762"` | Url on which to connect to a rabbitmq instance. Includes `amqp://` protocol |
| NOTIFICATION_EXCHANGE | `survey_notify`                      | Name of rabbit exchange to publish notifications to |
