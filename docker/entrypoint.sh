#!/bin/sh
# Start the app as an unprivileged user.
#
# The image used to run as root, so the folders docker compose bind-mounts from the
# host (./data, ./outputs, ./favorites) hold files that root created. An app that
# no longer is root could not write to them after an update. This script runs as
# root only long enough to hand the root-owned entries of those folders to the
# app's user, then drops root for good.
#
#   TTS_WEBUI_RUN_AS_ROOT=1   skip all of this and run as root, as the image used to
#
# Only entries owned by root are changed. A folder that belongs to another host user
# is left alone (taking over a host user's files would lock them out of their own
# folder); the script says so, and the folder has to be made writable for the app's
# user (uid 1000) by whoever owns it.
set -eu

APP_USER=appuser
APP_ROOT=${TTS_WEBUI_ROOT:-/app/tts-webui}

if [ "$(id -u)" != 0 ] || [ "${TTS_WEBUI_RUN_AS_ROOT:-}" = 1 ]; then
  exec "$@"
fi

uid=$(id -u "$APP_USER")
gid=$(id -g "$APP_USER")

as_app() { setpriv --reuid="$uid" --regid="$gid" --init-groups -- "$@"; }

for d in data outputs favorites voices collections; do
  p=$APP_ROOT/$d
  if [ ! -d "$p" ] || [ -L "$p" ]; then
    continue
  fi
  if [ -n "$(find "$p" -xdev -user 0 -print -quit 2>/dev/null || true)" ]; then
    echo "tts-webui: handing the root-owned files in $p to $APP_USER (uid $uid); this happens once, after an update from an image that ran as root" >&2
    if ! find "$p" -xdev -user 0 -exec chown -h "$uid:$gid" {} + 2>/dev/null; then
      echo "tts-webui: warning: could not change the owner of everything in $p (a network or Windows-backed mount does not allow it)" >&2
    fi
  fi
  if ! as_app test -w "$p"; then
    echo "tts-webui: warning: $p is not writable by $APP_USER (uid $uid); make it writable for that uid, or set TTS_WEBUI_RUN_AS_ROOT=1 to run as root" >&2
  fi
done

export HOME="/home/$APP_USER" USER="$APP_USER" LOGNAME="$APP_USER"
exec setpriv --reuid="$uid" --regid="$gid" --init-groups --no-new-privs -- "$@"
