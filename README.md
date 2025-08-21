# Insider Messaging Service

This project is RESTful API developed using Golang (messaging-service). Also it has "Background Job" to send messages automatically. Project is fully dockerized.

* To test APIs with swagger: http://localhost:8080/swagger/index.html
* After create message(s) with `POST api/messages`, job will send messages.
* Activate or deactivate with `PUT api/messages/toggle-job`. Active by default
* Also retrieve sent messages with `GET api/messages/sent-messages`

### Technologies
* Go 1.24
* Gin Framework
* PostgreSQL
* Redis Cache
* Swagger (Swaggo)

## Setup

1.  Clone the repository:

    ```bash
    git clone https://github.com/fbozkurt1/insider-message-service.git
    
    cd insider-message-service
    ```

2.  Need the Docker to run:

    ```bash
    docker-compose up -d
    ```
3. Open link to test: http://localhost:8080/swagger/index.html