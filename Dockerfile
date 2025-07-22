FROM alpine:latest

# Create a dummy file
RUN echo "This is a dummy file content" > /dummy.txt

# Set working directory
WORKDIR /

# Command to run when container starts
CMD ["cat", "/dummy.txt"]