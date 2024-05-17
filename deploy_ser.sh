#!/bin/bash

# Define variables
NEW_IMAGE="026450499422.dkr.ecr.us-east-1.amazonaws.com/stock-portfolio-api:latest"
SHARED_OUTPUTS_FILE="../infrastructor/shared-stack-outputs.json"
CLUSTER_NAME=$(jq -r '.[] | select(.OutputKey=="ECSClusterName") | .OutputValue' $SHARED_OUTPUTS_FILE)
SERVICE_NAME="stock-portfolio-api-service"
TASK_FAMILY="stock-portfolio-td"
CONTAINER_NAME="stock-portfolio-container"

# Fetch the latest ACTIVE task definition ARN and definition
TASK_DEFINITION=$(aws ecs describe-task-definition --task-definition $TASK_FAMILY --query 'taskDefinition.taskDefinitionArn' --output text)

# Create a new task definition revision with the new image
NEW_TASK_DEF=$(aws ecs register-task-definition \
    --family $(aws ecs describe-task-definition --task-definition $TASK_DEFINITION --query 'taskDefinition.family' --output text) \
    --container-definitions "[{\"name\": \"$CONTAINER_NAME\", \"image\": \"$NEW_IMAGE\", \"cpu\": 0, \"memory\": 256, \"essential\": true, \"portMappings\": [{\"containerPort\": 8080, \"hostPort\": 8080}]}]" \
    --requires-compatibilities $(aws ecs describe-task-definition --task-definition $TASK_DEFINITION --query 'taskDefinition.requiresCompatibilities' --output text) \
    --network-mode $(aws ecs describe-task-definition --task-definition $TASK_DEFINITION --query 'taskDefinition.networkMode' --output text) \
    --cpu $(aws ecs describe-task-definition --task-definition $TASK_DEFINITION --query 'taskDefinition.cpu' --output text) \
    --memory $(aws ecs describe-task-definition --task-definition $TASK_DEFINITION --query 'taskDefinition.memory' --output text) \
    --execution-role-arn $(aws ecs describe-task-definition --task-definition $TASK_DEFINITION --query 'taskDefinition.executionRoleArn' --output text) \
    --query 'taskDefinition.taskDefinitionArn' --output text)

# Update ECS service
aws ecs update-service --cluster $CLUSTER_NAME --service $SERVICE_NAME --task-definition $NEW_TASK_DEF

echo "Service updated to use new image: $NEW_IMAGE"

# Wait for the service to stabilize
echo "Waiting for service to stabilize..."
aws ecs wait services-stable --cluster $CLUSTER_NAME --services $SERVICE_NAME
if [ $? -eq 0 ]; then
    echo "Deployment successful. Service is stable."
else
    echo "Deployment failed or timed out."
fi
