FROM postgres:16-alpine

COPY init.sh /usr/local/bin/replica-init.sh

RUN chmod +x /usr/local/bin/replica-init.sh

ENTRYPOINT ["/usr/local/bin/replica-init.sh"]