FROM androsh7/nuitka-compiler:latest-x86_64-glibc-2.28-py3.13
WORKDIR /src
COPY cloud_command /src/cloud_command
COPY pyproject.toml /src/pyproject.toml
COPY version.txt /src/version.txt
RUN mkdir -p /src/frontend
COPY frontend/dist /src/frontend/dist
RUN python3 -m pip install .[dev] \
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