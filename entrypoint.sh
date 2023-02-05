#!/bin/sh

if [ -z "$AUTH_TYPE" ]; then
  echo "AUTH_TYPE is required"
  echo "current support basic"
fi

gztemplate ${AUTH_TYPE}
