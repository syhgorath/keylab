# Gerçek bir Ubuntu 26.04 sunucusuna olabildiğince yakın bir LAB hedefi (systemd + sshd + sudo + ufw).
# Yalnızca yerel testler içindir; --privileged çalıştırılır ve Colima/Docker VM'inin çekirdeğini paylaşır.
FROM ubuntu:26.04
ENV DEBIAN_FRONTEND=noninteractive
# Paket listeleri BİLEREK silinmez: gerçek bir sunucuda da dururlar (aksi halde apt paketleri bulamaz).
RUN apt-get update \
 && apt-get install -y --no-install-recommends systemd systemd-sysv dbus openssh-server sudo python3 python3-apt \
        iproute2 ufw iptables kmod procps ca-certificates \
 && apt-get update \
 && useradd -m -s /bin/bash tester \
 && echo 'tester ALL=(ALL) NOPASSWD:ALL' > /etc/sudoers.d/tester && chmod 440 /etc/sudoers.d/tester \
 && mkdir -p /home/tester/.ssh && chmod 700 /home/tester/.ssh && chown tester:tester /home/tester/.ssh \
 && systemctl enable ssh \
 && systemctl set-default multi-user.target

# --- LAB UYUM KATMANI --------------------------------------------------------------------------------
# Konteynerde olmayan tek şey çekirdek denetim (audit netlink) alt sistemidir: auditd başlamaz ve
# `augenrules --load` başarısız olur. Bu iki noktayı taklit ederiz; geri kalan her şey (apt, systemd,
# ufw, fail2ban, AIDE, sshd drop-in, sysctl) gerçek çalışır. Bu fark gerçek sunucuda YOKTUR.
RUN mkdir -p /etc/systemd/system/auditd.service.d \
 && printf '[Service]\nType=oneshot\nRemainAfterExit=yes\nExecStart=\nExecStart=/bin/true\nExecStartPost=\nExecStop=\nExecStopPost=\nExecReload=\n' \
        > /etc/systemd/system/auditd.service.d/lab.conf \
 && printf '#!/bin/sh\nexit 0\n' > /usr/local/sbin/augenrules && chmod 755 /usr/local/sbin/augenrules

STOPSIGNAL SIGRTMIN+3
CMD ["/sbin/init"]
