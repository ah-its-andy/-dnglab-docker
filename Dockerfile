FROM ubuntu:latest

ENV DATA_DIR "/etc/dnglab-docker/data"
ENV SOURCE_DIR "/home/dnglab/source"
ENV DEST_DIR "/home/dnglab/output"
ENV FILE_EXTS ".arw"

WORKDIR /etc/dnglab-docker/app

ADD dnglab_0.5.2_amd64.deb dnglab.deb

RUN dpkg -i dnglab.deb

ADD dnglab-docker dnglab-docker

CMD ["dnglab-docker"]