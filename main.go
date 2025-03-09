package main

import (
	"context"
	"encoding/json"
	"log"
	"net/http"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"

	"go.opentelemetry.io/contrib/instrumentation/github.com/aws/aws-lambda-go/otellambda"
	"go.opentelemetry.io/contrib/instrumentation/github.com/aws/aws-lambda-go/otellambda/xrayconfig"
	"go.opentelemetry.io/contrib/instrumentation/github.com/aws/aws-sdk-go-v2/otelaws"
	"go.opentelemetry.io/contrib/propagators/aws/xray"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/trace"
)

var (
	db        *dynamodb.Client
	tableName = "<YOUR_DYNAMODB_TABLE_NAME>"
	tracer    trace.Tracer
)

type Shot struct {
	ID         string  `json:"id" dynamodbav:"id"`
	PlayerID   string  `json:"player_id" dynamodbav:"player_id"`
	Player     string  `json:"player" dynamodbav:"player"`
	Team       string  `json:"team" dynamodbav:"team"`
	GameDate   string  `json:"game_date" dynamodbav:"game_date"`
	Quarter    int     `json:"quarter" dynamodbav:"quarter"`
	TimeLeft   string  `json:"time_left" dynamodbav:"time_left"`
	X          float64 `json:"x" dynamodbav:"x"`
	Y          float64 `json:"y" dynamodbav:"y"`
	ShotType   string  `json:"shot_type" dynamodbav:"shot_type"`
	Outcome    string  `json:"outcome" dynamodbav:"outcome"`
	ActionType string  `json:"action_type" dynamodbav:"action_type"`
	BasicZone  string  `json:"basic_zone" dynamodbav:"basic_zone"`
	ShotsMade  int64   `json:"shots_made" dynamodbav:"shots_made"`
}

func initAWS(ctx context.Context) {
	log.Println("Initializing AWS SDK with OpenTelemetry instrumentation")

	cfg, err := config.LoadDefaultConfig(ctx)
	if err != nil {
		log.Fatalf("Error loading AWS SDK config: %v", err)
	}

	// Instrument AWS SDK with OpenTelemetry
	otelaws.AppendMiddlewares(&cfg.APIOptions, otelaws.WithTracerProvider(otel.GetTracerProvider()))
	db = dynamodb.NewFromConfig(cfg)

	log.Println("AWS SDK initialized successfully")
}

func handler(ctx context.Context, request events.APIGatewayProxyRequest) (events.APIGatewayProxyResponse, error) {
	ctx, span := tracer.Start(ctx, "LambdaHandler")
	defer span.End()

	log.Printf("Received %s request for %s", request.HTTPMethod, request.Resource)

	switch request.HTTPMethod {
	case "GET":
		if request.Resource == "/shots" {
			return getShots(ctx)
		} else if request.Resource == "/shots/{player_id}" {
			playerID := request.PathParameters["player_id"]
			return getShotsByPlayer(ctx, playerID)
		}
	case "POST":
		if request.Resource == "/shots" {
			return postShot(ctx, request.Body)
		}
	}

	log.Println("Invalid request received")
	return events.APIGatewayProxyResponse{
		StatusCode: http.StatusNotFound,
		Body:       `{"message": "Not Found"}`,
		Headers:    map[string]string{"Content-Type": "application/json"},
	}, nil
}

func getShots(ctx context.Context) (events.APIGatewayProxyResponse, error) {
	ctx, span := tracer.Start(ctx, "GetAllShots")
	defer span.End()

	log.Println("Fetching all shots from DynamoDB")

	input := &dynamodb.ScanInput{TableName: aws.String(tableName)}
	result, err := db.Scan(ctx, input)
	if err != nil {
		log.Printf("DynamoDB Scan error: %v", err)
		return serverError("Failed to fetch data")
	}

	var shots []Shot
	if err := attributevalue.UnmarshalListOfMaps(result.Items, &shots); err != nil {
		log.Printf("Unmarshal error: %v", err)
		return serverError("Failed to unmarshal data")
	}

	log.Printf("Fetched %d shots", len(shots))
	return jsonResponse(http.StatusOK, shots)
}

func getShotsByPlayer(ctx context.Context, playerID string) (events.APIGatewayProxyResponse, error) {
	ctx, span := tracer.Start(ctx, "GetShotsByPlayer")
	defer span.End()

	log.Printf("Fetching shots for player ID: %s", playerID)

	input := &dynamodb.QueryInput{
		TableName:              aws.String(tableName),
		IndexName:              aws.String("player_idIndex"),
		KeyConditionExpression: aws.String("player_id = :player_id"),
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":player_id": &types.AttributeValueMemberS{Value: playerID},
		},
	}

	result, err := db.Query(ctx, input)
	if err != nil {
		log.Printf("Query error: %v", err)
		return serverError("Failed to query shots")
	}

	var playerShots []Shot
	if err := attributevalue.UnmarshalListOfMaps(result.Items, &playerShots); err != nil {
		log.Printf("Unmarshal error: %v", err)
		return serverError("Failed to process response")
	}

	return jsonResponse(http.StatusOK, playerShots)
}

func postShot(ctx context.Context, body string) (events.APIGatewayProxyResponse, error) {
	ctx, span := tracer.Start(ctx, "PostShot")
	defer span.End()

	log.Println("Processing POST request")

	var shot Shot
	if err := json.Unmarshal([]byte(body), &shot); err != nil {
		log.Printf("Unmarshal error: %v", err)
		return clientError("Invalid input data")
	}

	input := &dynamodb.PutItemInput{
		TableName: aws.String(tableName),
		Item: map[string]types.AttributeValue{
			"id":        &types.AttributeValueMemberS{Value: shot.ID},
			"player_id": &types.AttributeValueMemberS{Value: shot.PlayerID},
			"player":    &types.AttributeValueMemberS{Value: shot.Player},
		},
	}

	if _, err := db.PutItem(ctx, input); err != nil {
		log.Printf("PutItem error: %v", err)
		return serverError("Failed to add shot")
	}

	return events.APIGatewayProxyResponse{
		StatusCode: http.StatusOK,
		Body:       `{"message": "Shot added successfully"}`,
	}, nil
}

func main() {
	ctx := context.Background()

	// Initialize OpenTelemetry first
	tp, err := xrayconfig.NewTracerProvider(ctx)
	if err != nil {
		log.Fatalf("Failed to create tracer provider: %v", err)
	}
	defer func() {
		if err := tp.Shutdown(ctx); err != nil {
			log.Printf("Error shutting down tracer provider: %v", err)
		}
	}()

	otel.SetTracerProvider(tp)
	otel.SetTextMapPropagator(xray.Propagator{})
	tracer = otel.Tracer("nba-shots-api")

	// Initialize AWS SDK after OpenTelemetry
	initAWS(ctx)

	// Configure Lambda handler with OpenTelemetry
	lambda.Start(otellambda.InstrumentHandler(handler,
		xrayconfig.WithRecommendedOptions(tp)...))
}

// Helper functions
func jsonResponse(status int, data interface{}) (events.APIGatewayProxyResponse, error) {
	body, _ := json.Marshal(data)
	return events.APIGatewayProxyResponse{
		StatusCode: status,
		Body:       string(body),
		Headers:    map[string]string{"Content-Type": "application/json"},
	}, nil
}

func serverError(msg string) (events.APIGatewayProxyResponse, error) {
	return jsonResponse(http.StatusInternalServerError, map[string]string{"error": msg})
}

func clientError(msg string) (events.APIGatewayProxyResponse, error) {
	return jsonResponse(http.StatusBadRequest, map[string]string{"error": msg})
}
