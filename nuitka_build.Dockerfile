ARG architecture="x86_64"
ARG libc="glibc-2.28"
FROM androsh7/nuitka-compiler:latest-${architecture}-${libc}-py3.13
WORKDIR /src
COPY cloud_command /src/cloud_command
COPY pyproject.toml /src/pyproject.toml
COPY version.txt /src/version.txt
COPY boto_clean.py /src/boto_clean.py
RUN mkdir -p /src/frontend
COPY frontend/dist /src/frontend/dist
ENV CLEAN_BOTO_IGNORE_ENV=True
RUN python3 -m pip install .[dev] \
&& python3 /src/boto_clean.py \
&& python3 -m nuitka \
    --standalone \
    --onefile \
    --output-filename=/src/cloud_command.bin \
    --onefile-tempdir-spec={HOME}/.cloud_command \
    --include-data-file=version.txt=version.txt \
    --include-data-dir=frontend/dist=frontend/dist \
    --include-module=cloud_command.app \
    /src/cloud_command/runner.py
ENTRYPOINT [ "/src/cloud_command.bin", "--version" ]
