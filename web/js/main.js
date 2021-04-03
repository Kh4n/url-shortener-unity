document.addEventListener("DOMContentLoaded", function() {
    document.getElementById("submit").addEventListener("click", shorten, false)
})

function encodeForm(obj) {
    var str = [];
    for (var key in obj) {
         if (obj.hasOwnProperty(key)) {
               str.push(encodeURIComponent(key) + "=" + encodeURIComponent(obj[key]))
         }
    }
    return str.join("&");
}

function shorten() {
    base = window.location.origin
    input = document.getElementById("input")
    output = document.getElementById("output")
    output.innerHTML = ""
    var xhr = new XMLHttpRequest()
    xhr.open("POST", "/api/shorten", true)
    xhr.onreadystatechange = function() {
        if (xhr.readyState == 4 && xhr.status == 200) {
            resp = JSON.parse(xhr.responseText)
            if (resp.succeeded) {
                console.log(resp)
                link = new URL(resp.key, base)
                output.innerHTML = "URL: "
                anchor = document.createElement("a")
                anchor.href = link.href
                anchor.innerHTML = link.href
                output.appendChild(anchor)
            } else {
                output.innerHTML = "Request failed: " + resp.errorMsg
            }
        }
    }
    xhr.setRequestHeader("Content-Type", "application/x-www-form-urlencoded");
    xhr.send(encodeForm({url:input.value}))
}