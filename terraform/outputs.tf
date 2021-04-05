output "dbserver_ip" {
  value       = aws_instance.servers[0].public_ip
  description = "public ip of the database server"
}

output "cache_ip" {
  value       = aws_instance.servers[1].public_ip
  description = "public ip of the cache server"
}

output "webapp_ip" {
  value       = aws_instance.servers[2].public_ip
  description = "public ip of the cache server"
}
