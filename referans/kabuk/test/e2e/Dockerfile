# Gerçek Ubuntu gibi davranan, yalnızca yerel test için bir sshd.
# root hesabı kilitli gelir (Ubuntu varsayılanı); şifreyle yalnızca 'tester' girebilir.
FROM ubuntu:24.04

RUN apt-get update \
 && apt-get install -y --no-install-recommends openssh-server \
 && rm -rf /var/lib/apt/lists/* \
 && mkdir -p /run/sshd \
 && useradd -m -s /bin/bash tester \
 # konteynerde pam_loginuid oturum açmayı bozabiliyor
 && sed -i 's@session\s*required\s*pam_loginuid.so@session optional pam_loginuid.so@' /etc/pam.d/sshd

COPY entrypoint.sh /entrypoint.sh
RUN chmod 0755 /entrypoint.sh

EXPOSE 22
ENTRYPOINT ["/entrypoint.sh"]
