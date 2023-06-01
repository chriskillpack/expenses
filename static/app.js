function linkSuccess(public_token, metadata) {
    // This is where we call the server to exchange the public token for an access token
    fetch("/get_access_token", {
        method: 'POST',
        body: JSON.stringify({
            public_token: public_token,
            accounts: metadata.accounts,
            institution: metadata.institution,
            link_session_id: metadata.link_session_id,
        }),
        headers: {
            "Content-Type": "application/json",
        }
    });
}

window.addEventListener("load", (event) => {
    const el = document.querySelector("#start");
    el.addEventListener("click", (event) => {
        fetch("/link/token/create", {method: 'POST'})
            .then((response) => response.json())
            .then((res) => {
                Plaid.create({
                    token: res.LinkToken,
                    onSuccess: linkSuccess,
                    onExit: (err, metadata) => {
                        console.error(err, metadata);
                    },
                    onEvent: (eventName, metadata) => {
                        console.log("Event:", eventName);
                        console.log("Metadata:", metadata);
                    },
                }).open()
            })
            .catch(console.error)
    });
});
