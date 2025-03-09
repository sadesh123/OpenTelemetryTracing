# NBA Player Data API with OpenTelemetry and AWS X-Ray

This project is a Go-based serverless API that retrieves NBA player shot data from DynamoDB, providing insights into player performance. The API is built using AWS Lambda, API Gateway, and DynamoDB. It is instrumented with **OpenTelemetry** for distributed tracing, integrated with **AWS X-Ray** to monitor performance and troubleshoot latency issues.

## Features

- **Retrieve all NBA shots**: Get data on all shots made by players in the dataset.
- **Retrieve shots by player**: Query the database for shots made by a specific player using their player ID.
- **Add new shot data**: Submit new shot data to the database through a POST request.

## Technology Stack

- **Go**: Programming language for building the API.
- **AWS Lambda**: Serverless compute service to run the API.
- **AWS API Gateway**: API Gateway to expose the Lambda function via HTTP endpoints.
- **AWS DynamoDB**: NoSQL database to store NBA player shot data.
- **OpenTelemetry**: Instrumentation for distributed tracing and monitoring.
- **AWS X-Ray**: Trace and visualize API performance and latency.

## Installation

To get started with this project, follow the steps below:

1. Clone this repository to your local machine:

   ```bash
   git clone https://github.com/sadesh123/nba-player-data-api.git

