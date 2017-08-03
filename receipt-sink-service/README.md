Receipt Sink Service
====================

Acts as a receipting service to pick simulate a service that listens for survey
notifications to receipt a transaction.

Picks notifications of a queue and logs the receipt - then discards them.

## Environment

Expects the following environment to be set:

| Var                   | Example                              | Description                                              |
| --------------------- | ------------------------------------ | -------------------------------------------------------- |
| PORT                  | `"5000"`                             | String describing the port on which to start the service |
| RABBIT_URL            | `"amqp://rabbit:rabbit@rabbit:5762"` | Url on which to connect to a rabbitmq instance. Includes `amqp://` protocol |
| NOTIFICATION_EXCHANGE | `survey_notify`                      | Name of rabbit exchange to publish notifications to |