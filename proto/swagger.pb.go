package proto 

const (
swagger = `{
  "swagger": "2.0",
  "info": {
    "title": "proto/service.proto",
    "version": "version not set"
  },
  "schemes": [
    "http",
    "https"
  ],
  "consumes": [
    "application/json"
  ],
  "produces": [
    "application/json"
  ],
  "paths": {
    "/v1/echo": {
      "post": {
        "operationId": "Echo",
        "responses": {
          "200": {
            "description": "",
            "schema": {
              "$ref": "#/definitions/protoEchoMessage"
            }
          }
        },
        "parameters": [
          {
            "name": "body",
            "in": "body",
            "required": true,
            "schema": {
              "$ref": "#/definitions/protoEchoMessage"
            }
          }
        ],
        "tags": [
          "EchoService"
        ]
      }
    }
  },
  "definitions": {
    "protoEchoMessage": {
      "type": "object",
      "properties": {
        "value": {
          "type": "string"
        }
      }
    }
  }
}
`
)
