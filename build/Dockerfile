FROM registry.access.redhat.com/ubi8/ubi:latest

ENV OPERATOR=/usr/local/bin/mysql-operator \
    USER_UID=1001 \
    USER_NAME=mysql-operator



# install operator binary
COPY build/_output/bin/mysql-operator ${OPERATOR}

COPY build/bin /usr/local/bin

RUN yum install -y https://dev.mysql.com/get/mysql80-community-release-el8-1.noarch.rpm && \
    yum install -y mysql-community-client mysql-shell && \
    /usr/local/bin/user_setup

ENTRYPOINT ["/usr/local/bin/entrypoint"]

USER ${USER_UID}
