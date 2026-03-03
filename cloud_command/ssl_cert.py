"""Functions for creating a self-signed certificate"""

# Standard libraries
import ipaddress
from datetime import UTC, datetime, timedelta

# Third-party libraries
from cryptography import x509
from cryptography.hazmat.primitives import hashes, serialization
from cryptography.hazmat.primitives.asymmetric import rsa
from cryptography.x509.oid import NameOID

# Project libraries
from cloud_command.constants import GENERATED_SSL_CERTIFICATE, GENERATED_SSL_KEY


def generate_self_signed_cert():
    common_name = "Cloud Command"

    key = rsa.generate_private_key(public_exponent=65537, key_size=4096)

    subject = issuer = x509.Name([x509.NameAttribute(NameOID.COMMON_NAME, common_name)])

    cert = (
        x509.CertificateBuilder()
        .subject_name(subject)
        .issuer_name(issuer)
        .public_key(key.public_key())
        .serial_number(x509.random_serial_number())
        .not_valid_before(datetime.now(UTC))
        .not_valid_after(datetime.now(UTC) + timedelta(days=365 * 10))
        .add_extension(
            x509.SubjectAlternativeName(
                [
                    x509.DNSName(common_name),
                    x509.DNSName("localhost"),
                    x509.IPAddress(ipaddress.IPv4Address("127.0.0.1")),
                ]
            ),
            critical=False,
        )
        .sign(key, hashes.SHA256())
    )

    with open(GENERATED_SSL_KEY, "wb") as key_file:
        key_file.write(
            key.private_bytes(
                serialization.Encoding.PEM,
                serialization.PrivateFormat.TraditionalOpenSSL,
                serialization.NoEncryption(),
            )
        )

    with open(GENERATED_SSL_CERTIFICATE, "wb") as cert_file:
        cert_file.write(cert.public_bytes(serialization.Encoding.PEM))
