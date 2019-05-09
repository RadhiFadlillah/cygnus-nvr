var template = `
<div id="page-live">
    <div class="page-header">
        <p>Live Stream</p>
        <a href="#" title="Refresh storage" @click="loadCameras">
            <i class="fas fa-fw fa-sync-alt"></i>
        </a>
        <a href="#" title="Add camera" @click="showDialogNewCamera">
            <i class="fas fa-fw fa-plus-circle"></i>
        </a>
    </div>
    <div class="video-container">
        <video-player v-for="(cam, idx) in cameras" :key=idx source="/cam/1/live/playlist">
        </video-player>
    </div>
    <div class="loading-overlay" v-if="loading"><i class="fas fa-fw fa-spin fa-spinner"></i></div>
    <cygnus-dialog v-bind="dialog"/>
</div>`;

import cygnusDialog from "../component/dialog.js";
import videoPlayer from "../component/video-player.js";
import basePage from "./base.js";

export default {
    template: template,
    mixins: [basePage],
    components: {
        cygnusDialog,
        videoPlayer
    },
    data() {
        return {
            cameras: [],
            loading: false,
        }
    },
    methods: {
        loadCameras() {
            this.loading = true;

            fetch("/api/camera")
                .then(response => {
                    if (!response.ok) throw response;
                    return response.json();
                })
                .then(json => {
                    console.log(json);
                    this.cameras = json;
                    this.loading = false;
                })
                .catch(err => {
                    this.loading = false;
                    err.text().then(msg => {
                        this.showErrorDialog(`${msg} (${err.status})`);
                    })
                });
        },
        showDialogNewCamera() {
            this.showDialog({
                title: "New Camera",
                content: "Input new camera's data :",
                fields: [{
                    name: "url",
                    label: "Domain URL",
                    value: "",
                }, {
                    name: "name",
                    label: "Camera's name",
                    value: "",
                }, {
                    name: "username",
                    label: "Username",
                    value: "",
                }, {
                    name: "password",
                    label: "Password",
                    type: "password",
                    value: "",
                }, {
                    name: "repeat",
                    label: "Repeat password",
                    type: "password",
                    value: "",
                }],
                mainText: "OK",
                secondText: "Cancel",
                mainClick: (data) => {
                    if (data.url === "") {
                        this.showErrorDialog("Domain URL must not empty");
                        return;
                    }

                    if (data.name === "") {
                        this.showErrorDialog("Camera's name must not empty");
                        return;
                    }

                    if (data.username === "") {
                        this.showErrorDialog("Username must not empty");
                        return;
                    }

                    if (data.password === "") {
                        this.showErrorDialog("Password must not empty");
                        return;
                    }

                    if (data.password !== data.repeat) {
                        this.showErrorDialog("Password does not match");
                        return;
                    }

                    this.dialog.loading = true;
                    fetch("/api/camera", {
                            method: "post",
                            body: JSON.stringify(data),
                            headers: {
                                "Content-Type": "application/json",
                            },
                        })
                        .then(response => {
                            if (!response.ok) throw response;
                            return response;
                        })
                        .then(() => {
                            this.dialog.loading = false;
                            this.dialog.visible = false;
                        })
                        .catch(err => {
                            this.dialog.loading = false;
                            err.text().then(msg => {
                                this.showErrorDialog(`${msg} (${err.status})`);
                            })
                        });
                }
            });
        }
    },
    mounted() {
        this.loadCameras();
    }
}