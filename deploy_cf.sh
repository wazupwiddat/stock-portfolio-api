#!/bin/bash

# Set variables
STACK_NAME="stock-portfolio-api-stack"
TEMPLATE_FILE="cloudformation.yaml"
ECR_IMAGE_URI="026450499422.dkr.ecr.us-east-1.amazonaws.com/stock-portfolio-api:latest"
SHARED_OUTPUTS_FILE="../infrastructor/shared-stack-outputs.json"
CERT_ARN="arn:aws:acm:us-east-1:026450499422:certificate/390e9185-4df5-4405-89d3-38ec9e090bac"


# Read the shared outputs from the JSON file
VPC_ID=$(jq -r '.[] | select(.OutputKey=="VPCId") | .OutputValue' $SHARED_OUTPUTS_FILE)
PUBLIC_SUBNET_ONE_ID=$(jq -r '.[] | select(.OutputKey=="PublicSubnetOneId") | .OutputValue' $SHARED_OUTPUTS_FILE)
PUBLIC_SUBNET_TWO_ID=$(jq -r '.[] | select(.OutputKey=="PublicSubnetTwoId") | .OutputValue' $SHARED_OUTPUTS_FILE)
SECURITY_GROUP_ID=$(jq -r '.[] | select(.OutputKey=="SecurityGroupId") | .OutputValue' $SHARED_OUTPUTS_FILE)
ALB_ARN=$(jq -r '.[] | select(.OutputKey=="ALBArn") | .OutputValue' $SHARED_OUTPUTS_FILE)
HTTP_LISTENER_ARN=$(jq -r '.[] | select(.OutputKey=="HTTPListenerArn") | .OutputValue' $SHARED_OUTPUTS_FILE)
HTTPS_LISTENER_ARN=$(jq -r '.[] | select(.OutputKey=="HTTPSListenerArn") | .OutputValue' $SHARED_OUTPUTS_FILE)
ECS_CLUSTER_NAME=$(jq -r '.[] | select(.OutputKey=="ECSClusterName") | .OutputValue' $SHARED_OUTPUTS_FILE)
ECS_TASK_EXECUTION_ROLE_ARN=$(jq -r '.[] | select(.OutputKey=="ECSTaskExecutionRoleArn") | .OutputValue' $SHARED_OUTPUTS_FILE)

# Create the stack
aws cloudformation create-stack \
  --stack-name $STACK_NAME \
  --template-body file://$TEMPLATE_FILE \
  --parameters \
    ParameterKey=CertificateArn,ParameterValue=$CERT_ARN \
    ParameterKey=ECRImageURI,ParameterValue=$ECR_IMAGE_URI \
    ParameterKey=ALBArn,ParameterValue=$ALB_ARN \
    ParameterKey=HTTPListenerArn,ParameterValue=$HTTP_LISTENER_ARN \
    ParameterKey=HTTPSListenerArn,ParameterValue=$HTTPS_LISTENER_ARN \
    ParameterKey=ECSClusterName,ParameterValue=$ECS_CLUSTER_NAME \
    ParameterKey=PublicSubnetOne,ParameterValue=$PUBLIC_SUBNET_ONE_ID \
    ParameterKey=PublicSubnetTwo,ParameterValue=$PUBLIC_SUBNET_TWO_ID \
    ParameterKey=SecurityGroup,ParameterValue=$SECURITY_GROUP_ID \
    ParameterKey=VpcId,ParameterValue=$VPC_ID \
    ParameterKey=ECSTaskExecutionRoleArn,ParameterValue=$ECS_TASK_EXECUTION_ROLE_ARN \
  --capabilities CAPABILITY_NAMED_IAM

# Wait for the stack to be created
aws cloudformation wait stack-create-complete --stack-name $STACK_NAME

# Check if the stack creation was successful
if [ $? -eq 0 ]; then
  echo "Stack $STACK_NAME created successfully."
else
  echo "Failed to create stack $STACK_NAME."
  exit 1
fi

# Get the outputs of the stack
outputs=$(aws cloudformation describe-stacks --stack-name $STACK_NAME --query "Stacks[0].Outputs")

# Display the outputs
echo "Stack Outputs:"
echo $outputs | jq '.'

# Optionally save the outputs to a file
echo $outputs | jq '.' > stock-portfolio-api-outputs.json
echo "Outputs saved to stock-portfolio-api-outputs.json"
