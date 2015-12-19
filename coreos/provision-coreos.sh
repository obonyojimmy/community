#!/bin/bash
#
# Nanocloud Community, a comprehensive platform to turn any application
# into a cloud solution.
#
# Copyright (C) 2015 Nanocloud Software
#
# This file is part of Nanocloud community.
#
# Nanocloud community is free software; you can redistribute it and/or modify
# it under the terms of the GNU Affero General Public License as
# published by the Free Software Foundation, either version 3 of the
# License, or (at your option) any later version.
#
# Nanocloud community is distributed in the hope that it will be useful,
# but WITHOUT ANY WARRANTY; without even the implied warranty of
# MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
# GNU Affero General Public License for more details.
#
# You should have received a copy of the GNU Affero General Public License
# along with this program.  If not, see <http://www.gnu.org/licenses/>.


git clone --depth 1 https://github.com/Nanocloud/community.git
cd community/dockerfiles
(
  mkdir repos_guacamole; cd repos_guacamole/
  git clone --depth 1 https://github.com/Nanocloud/noauth-logged.git
)

(
  mkdir repos_nanocloud; cd repos_nanocloud/

  git clone --depth 1 https://github.com/Nanocloud/nanocloud.git
  git clone --depth 1 https://github.com/Nanocloud/users.git
  git clone --depth 1 https://github.com/Nanocloud/iaas.git
  git clone --depth 1 https://github.com/Nanocloud/ldap.git
  git clone --depth 1 https://github.com/Nanocloud/history.git
  git clone --depth 1 https://github.com/Nanocloud/apps.git
)

curl --progress-bar -L "https://github.com/docker/compose/releases/download/1.4.2/docker-compose-$(uname -s)-$(uname -m)" > docker-compose
chmod +x docker-compose

mkdir -p postgres
./docker-compose build

docker pull glyptodon/guacd:0.9.8
docker pull nginx:1.9
docker pull cpuguy83/docker-grand-ambassador:0.9.1
docker pull rabbitmq:3.5
docker pull postgres:9.4

sudo cp nanocloud.service /etc/systemd/system/nanocloud.service
sudo systemctl enable /etc/systemd/system/nanocloud.service

sudo shutdown
