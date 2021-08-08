#!/bin/bash
set -e

host="https://chat.aaronbatilo.dev"

username=$(cat /dev/urandom | tr -dc 'a-zA-Z0-9' | fold -w 32 | head -n 1)
password=$(cat /dev/urandom | tr -dc 'a-zA-Z0-9' | fold -w 32 | head -n 1)

echo "Creating a user..."
user_id=$(curl -s --cookie-jar /tmp/cj --data "{\"username\":\"${username}\", \"password\":\"${password}\"}" --cookie /tmp/cj "${host}/users" | jq -r '.id')
echo "Created user ${user_id}"

echo "Logging in to get a valid session token..."
token=$(curl -s --cookie-jar /tmp/cj --data "{\"username\":\"${username}\", \"password\":\"${password}\"}" --cookie /tmp/cj "${host}/login" | jq -r '.token')
echo "Login was successful. We can send requests with ${token}"


for i in {1..10}
do
  message_type=$(cat /dev/urandom | tr -dc '0-2' | fold -w 512 | head -n 1 | head --bytes 1)
  if [ "${message_type}" == "0" ]; then
    echo "Creating text message..."
    text=$(cat /dev/urandom | tr -dc 'a-zA-Z0-9' | fold -w 32 | head -n 1)
    curl -H"Authorization: ${token}" --cookie-jar /tmp/cj --cookie /tmp/cj --data "{\"sender\":${user_id}, \"recipient\": 1, \"content\":{\"type\":\"text\",\"text\":\"${text}\"}}" "${host}/messages"
  fi

  if [ "${message_type}" == "1" ]; then
    echo "Creating image message..."
    url=$(cat /dev/urandom | tr -dc 'a-zA-Z0-9' | fold -w 32 | head -n 1)
    width=$(cat /dev/urandom | tr -dc '0-9' | fold -w 256 | head -n 1 | sed -e 's/^0*//' | head --bytes 3)
    height=$(cat /dev/urandom | tr -dc '0-9' | fold -w 256 | head -n 1 | sed -e 's/^0*//' | head --bytes 3)
    curl -H"Authorization: ${token}" --cookie-jar /tmp/cj --cookie /tmp/cj --data "{\"sender\":${user_id}, \"recipient\": 1, \"content\":{\"type\":\"image\",\"url\":\"${url}\", \"width\": ${width}, \"height\": ${height}}}" "${host}/messages"
  fi

  if [ "${message_type}" == "2" ]; then
    echo "Create video message..."
    url=$(cat /dev/urandom | tr -dc 'a-zA-Z0-9' | fold -w 32 | head -n 1)
    curl -H"Authorization: ${token}" --cookie-jar /tmp/cj --cookie /tmp/cj --data "{\"sender\":${user_id}, \"recipient\": 1, \"content\":{\"type\":\"video\",\"url\":\"${url}\", \"source\": \"youtube\"}}" "${host}/messages"
  fi
done

start=$(cat /dev/urandom | tr -dc '0-9' | fold -w 256 | head -n 1 | sed -e 's/^0*//' | head --bytes 2)
curl -s -X GET -H"Authorization: ${token}" --cookie-jar /tmp/cj --cookie /tmp/cj --data "{\"recipient\": 1, \"start\": ${start}, \"limit\": 100}" "${host}/messages" | jq -c '.messages[]'
