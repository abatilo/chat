#!/bin/bash
set -e

echo "Starting integration tests"

host="${HOST:-https://chat.aaronbatilo.dev}"

username=$(openssl rand -base64 12)
password=$(openssl rand -base64 12)

echo "Creating a user..."
user_id=$(curl -s --cookie-jar /tmp/cj --data "{\"username\":\"${username}\", \"password\":\"${password}\"}" --cookie /tmp/cj "${host}/users" | jq -r '.id')
echo "Created user ${user_id}"

echo "Logging in to get a valid session token..."
token=$(curl -s --cookie-jar /tmp/cj --data "{\"username\":\"${username}\", \"password\":\"${password}\"}" --cookie /tmp/cj "${host}/login" | jq -r '.token')
echo "Login was successful. We can send requests with ${token}"


for i in {1..10}
do
  message_type=$(echo $(( $RANDOM % 3 )))
  if [ "${message_type}" == "0" ]; then
    echo "Creating text message..."
    text=$(openssl rand -base64 12)
    curl -H"Authorization: ${token}" --cookie-jar /tmp/cj --cookie /tmp/cj --data "{\"sender\":${user_id}, \"recipient\": 1, \"content\":{\"type\":\"text\",\"text\":\"${text}\"}}" "${host}/messages"
  fi

  if [ "${message_type}" == "1" ]; then
    echo "Creating image message..."
    url=$(openssl rand -base64 12)
    width=$(echo $(( $RANDOM % 99 + 1 )))
    height=$(echo $(( $RANDOM % 99 + 1 )))
    curl -H"Authorization: ${token}" --cookie-jar /tmp/cj --cookie /tmp/cj --data "{\"sender\":${user_id}, \"recipient\": 1, \"content\":{\"type\":\"image\",\"url\":\"${url}\", \"width\": ${width}, \"height\": ${height}}}" "${host}/messages"
  fi

  if [ "${message_type}" == "2" ]; then
    echo "Create video message..."
    url=$(openssl rand -base64 12)
    curl -H"Authorization: ${token}" --cookie-jar /tmp/cj --cookie /tmp/cj --data "{\"sender\":${user_id}, \"recipient\": 1, \"content\":{\"type\":\"video\",\"url\":\"${url}\", \"source\": \"youtube\"}}" "${host}/messages"
  fi
done

start=$(echo $(( $RANDOM % 500 + 1 )))
curl -s -X GET -H"Authorization: ${token}" --cookie-jar /tmp/cj --cookie /tmp/cj --data "{\"recipient\": 1, \"start\": ${start}, \"limit\": 100}" "${host}/messages" | jq -c '.messages[]'
