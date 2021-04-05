provider "aws" {
  region     = "us-east-2"
  access_key = var.access_key
  secret_key = var.secret_key
}

resource "aws_vpc" "main" {
  cidr_block = "10.0.0.0/16"
}

resource "aws_internet_gateway" "gw" {
  vpc_id = aws_vpc.main.id
}

resource "aws_route_table" "default_route" {
  vpc_id = aws_vpc.main.id

  route {
    cidr_block = "0.0.0.0/0"
    gateway_id = aws_internet_gateway.gw.id
  }

  route {
    ipv6_cidr_block = "::/0"
    gateway_id      = aws_internet_gateway.gw.id
  }
}

resource "aws_subnet" "subnet" {
  vpc_id     = aws_vpc.main.id
  cidr_block = "10.0.1.0/24"

  availability_zone = "us-east-2a"
}

resource "aws_route_table_association" "a" {
  subnet_id      = aws_subnet.subnet.id
  route_table_id = aws_route_table.default_route.id
}

resource "aws_security_group" "allow_misc" {
  name        = "allow_misc"
  description = "Allow ssh, all egress, etc."
  vpc_id      = aws_vpc.main.id

  ingress {
    description = "SSH"
    from_port   = 22
    to_port     = 22
    protocol    = "tcp"
    cidr_blocks = ["0.0.0.0/0"]
  }
  egress {
    from_port   = 0
    to_port     = 0
    protocol    = "-1"
    cidr_blocks = ["0.0.0.0/0"]
  }
}

resource "aws_security_group" "allow_web" {
  name        = "allow_web_traffic"
  description = "Allow inbound traffic"
  vpc_id      = aws_vpc.main.id

  ingress {
    description = "HTTP from cache/webapp"
    from_port   = 80
    to_port     = 80
    protocol    = "tcp"
    cidr_blocks = ["0.0.0.0/0"]
  }
}

# resource "aws_network_interface" "net_int" {
#   subnet_id       = aws_subnet.subnet.id
#   private_ips     = ["10.0.1.1${count.index}"]
#   security_groups = [aws_security_group.allow_web.id, aws_security_group.allow_misc.id]
#   count           = 3
# }

# resource "aws_eip" "eip" {
#   vpc                       = true
#   network_interface         = aws_network_interface.net_int[count.index].id
#   associate_with_private_ip = "10.0.1.1${count.index}"
#   depends_on                = [aws_internet_gateway.gw]
#   count                     = 3
# }

resource "aws_instance" "servers" {
  ami           = "ami-05d72852800cbf29e"
  instance_type = "t2.micro"
  key_name      = "url-shortener-unity"

  associate_public_ip_address = true
  subnet_id                   = aws_subnet.subnet.id
  private_ip                  = "10.0.1.1${count.index}"
  vpc_security_group_ids      = [aws_security_group.allow_web.id, aws_security_group.allow_misc.id]

  count = 3
  connection {
    type        = "ssh"
    user        = "ec2-user"
    private_key = file(var.pem_file_loc)
    host        = self.public_ip
    timeout     = "1m"
  }

  provisioner "remote-exec" {
    script = "./install_docker.sh"
  }

  tags = {
    Name = "url-shortener-server-${count.index}"
  }
}

resource "null_resource" "dbserver_deploy" {
  connection {
    type        = "ssh"
    user        = "ec2-user"
    private_key = file(var.pem_file_loc)
    host        = aws_instance.servers[0].public_ip
    timeout     = "1m"
  }
  provisioner "file" {
    source      = "../docker/dbserver-compose.yml"
    destination = "~/dbserver-compose.yml"
  }
  provisioner "remote-exec" {
    inline = [
      "cd ~/", "mkdir -p badger-db",
      "export PORT=80",
      "docker-compose -f dbserver-compose.yml pull",
      "docker-compose -f dbserver-compose.yml up -d --remove-orphans"
    ]
  }
}

resource "null_resource" "cache_deploy" {
  connection {
    type        = "ssh"
    user        = "ec2-user"
    private_key = file(var.pem_file_loc)
    host        = aws_instance.servers[1].public_ip
    timeout     = "1m"
  }

  provisioner "file" {
    source      = "../docker/cache-compose.yml"
    destination = "~/cache-compose.yml"
  }
  provisioner "remote-exec" {
    script = "./install_docker.sh"
  }
  provisioner "remote-exec" {
    inline = [
      "cd ~/",
      "export PORT=80", "export RESERVE=1000",
      "export DBSERVER=${aws_instance.servers[0].public_ip}",
      "docker-compose -f cache-compose.yml pull",
      "docker-compose -f cache-compose.yml up -d --remove-orphans"
    ]
  }
}

resource "null_resource" "webapp_deploy" {
  connection {
    type        = "ssh"
    user        = "ec2-user"
    private_key = file(var.pem_file_loc)
    host        = aws_instance.servers[2].public_ip
    timeout     = "1m"
  }

  provisioner "file" {
    source      = "../docker/webapp-compose.yml"
    destination = "~/webapp-compose.yml"
  }
  provisioner "remote-exec" {
    script = "./install_docker.sh"
  }
  provisioner "remote-exec" {
    inline = [
      "cd ~/",
      "export PORT=80", "export BACKEND=${aws_instance.servers[1].public_ip}",
      "docker-compose -f webapp-compose.yml pull",
      "docker-compose -f webapp-compose.yml up -d --remove-orphans"
    ]
  }
}
