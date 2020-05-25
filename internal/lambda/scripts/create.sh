(
  cd ${TEMP_DIR}
  curl -SsL https://github.com/tkellen/memorybox/releases/download/${VERSION}/memorybox_linux_amd64.tar.gz | tar xzf - memorybox
  cat <<EOF > run.py
${SCRIPT}
EOF
  zip -r memorybox.zip run.py memorybox
  aws iam create-role \
    --role-name ${ROLE_NAME} \
    --assume-role-policy-document '{"Version": "2012-10-17","Statement": [{ "Effect": "Allow", "Principal": {"Service": "lambda.amazonaws.com"}, "Action": "sts:AssumeRole"}]}'
  aws iam attach-role-policy \
    --role-name ${ROLE_NAME} \
    --policy-arn arn:aws:iam::aws:policy/service-role/AWSLambdaBasicExecutionRole
  sleep 5
  aws lambda create-function \
    --function-name ${ROLE_NAME} \
    --runtime python3.8 \
    --role $(aws iam get-role --role-name ${ROLE_NAME} --output text --query='Role.Arn') \
    --zip-file=fileb://memorybox.zip \
    --handler run.main \
    --memory 3008 \
    --timeout 180
  rm memorybox.zip run.py memorybox
)