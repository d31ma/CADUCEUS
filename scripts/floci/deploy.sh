#!/usr/bin/env bash
set -euo pipefail
# shellcheck source-path=SCRIPTDIR
. "$(dirname "$0")/common.sh"

role_name() {
  echo "$(mesail_stack .functionPrefix)lambda"
}

role_arn() {
  echo "arn:aws:iam::$(mesail_stack .account):role/$(role_name)"
}

ensure_role() {
  if mesail_aws iam get-role --role-name "$(role_name)" > /dev/null 2>&1; then echo "role $(role_name) exists"; return; fi
  mesail_aws iam create-role --role-name "$(role_name)" \
    --assume-role-policy-document '{"Version":"2012-10-17","Statement":[{"Effect":"Allow","Principal":{"Service":"lambda.amazonaws.com"},"Action":"sts:AssumeRole"}]}'
}

file_system_id() {
  mesail_aws efs describe-file-systems --output text \
    --query "FileSystems[?CreationToken=='$(mesail_stack .fileSystem.creationToken)'].FileSystemId | [0]"
}

ensure_file_system() {
  if [ "$(file_system_id)" != None ]; then echo "file system $(file_system_id) exists"; return; fi
  mesail_aws efs create-file-system --creation-token "$(mesail_stack .fileSystem.creationToken)" \
    --tags Key=Name,Value=mesail-data Key=project,Value=mesail
}

access_point_arn() {
  local service="$1"
  mesail_aws efs create-access-point --client-token "mesail-$service" --file-system-id "$(file_system_id)" \
    --posix-user Uid=10001,Gid=10001 \
    --root-directory "Path=/$service,CreationInfo={OwnerUid=10001,OwnerGid=10001,Permissions=0750}" \
    --tags "Key=Name,Value=mesail-$service" Key=project,Value=mesail \
    --query AccessPointArn --output text
}

function_name() {
  echo "$(mesail_stack .functionPrefix)$1"
}

image_uri() {
  echo "$(mesail_stack .imagePrefix)$1:$MESAIL_IMAGE_TAG"
}

storage_arguments() {
  local service="$1"
  if [ "$(mesail_service_field "$service" fylo)" != true ]; then return; fi
  echo "--vpc-config SubnetIds=$(mesail_stack .network.subnet),SecurityGroupIds=$(mesail_stack .network.securityGroup)"
  echo "--file-system-configs Arn=$(access_point_arn "$service"),LocalMountPath=$(mesail_stack .fileSystem.mountPath)"
}

create_function() {
  local service="$1"
  # shellcheck disable=SC2046
  mesail_aws lambda create-function --function-name "$(function_name "$service")" \
    --package-type Image --code "ImageUri=$(image_uri "$service")" --role "$(role_arn)" \
    --architectures "$(mesail_stack .architecture)" \
    --memory-size "$(mesail_service_field "$service" memory)" --timeout "$(mesail_service_field "$service" timeout)" \
    --tags project=mesail,service="$service" \
    $(storage_arguments "$service") \
    --query '{name: FunctionName, state: State, image: Code.ImageUri}'
}

update_function() {
  local service="$1"
  mesail_aws lambda update-function-code --function-name "$(function_name "$service")" \
    --image-uri "$(image_uri "$service")" --query '{name: FunctionName, state: State}'
}

ensure_function() {
  local service="$1"
  if mesail_aws lambda get-function --function-name "$(function_name "$service")" > /dev/null 2>&1; then
    update_function "$service"
    return
  fi
  create_function "$service"
}

limit_writers() {
  local service="$1"
  if [ "$(mesail_service_field "$service" fylo)" != true ]; then return; fi
  mesail_aws lambda put-function-concurrency --function-name "$(function_name "$service")" --reserved-concurrent-executions 1
}

deploy_service() {
  local service="$1"
  ensure_function "$service"
  limit_writers "$service"
  mesail_aws lambda wait function-active-v2 --function-name "$(function_name "$service")"
  echo "$(function_name "$service") is active"
}

ensure_bucket() {
  local bucket
  bucket="$(mesail_stack .staticBucket)"
  if mesail_aws s3api head-bucket --bucket "$bucket" > /dev/null 2>&1; then echo "bucket $bucket exists"; return; fi
  mesail_aws s3api create-bucket --bucket "$bucket"
}

upload_static_app() {
  local app="$1" prefix="$2" folder
  folder="$(mesail_root)/apps/$app/dist/web"
  if [ ! -d "$folder" ]; then echo "warning: no bundle at $folder, so $app is not served"; return 0; fi
  mesail_aws s3 sync --delete "$folder" "s3://$(mesail_stack .staticBucket)/$prefix/"
}

upload_static_apps() {
  local app prefix
  while read -r app prefix; do
    upload_static_app "$app" "$prefix"
  done < <(mesail_stack '.staticApps[] | "\(.app) \(.prefix)"')
}

chosen_services() {
  if [ "$#" -gt 0 ]; then printf '%s\n' "$@"; return; fi
  mesail_services
}

main() {
  local service
  mesail_load_settings
  mesail_step deploy-role ensure_role
  mesail_step deploy-file-system ensure_file_system
  for service in $(chosen_services "$@"); do
    mesail_step "deploy-$service" deploy_service "$service"
  done
  mesail_step deploy-bucket ensure_bucket
  if [ "${MESAIL_SKIP_STATIC:-0}" = 1 ]; then echo "static upload skipped (MESAIL_SKIP_STATIC=1)"; return; fi
  mesail_step deploy-static upload_static_apps
}

main "$@"
