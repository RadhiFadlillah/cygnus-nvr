var template = `
<div class="cygnus-video-box">
    <video ref="videoPlayer" class="cygnus-video video-js">
        <source :src="url" type="application/x-mpegURL">
        <p class="vjs-no-js">
            To view this video please enable JavaScript, and consider upgrading to a web browser that
            <a href="https://videojs.com/html5-video-support/" target="_blank">supports HTML5 video</a>
        </p>
    </video>
    <div class="cygnus-video-menu">
        <p>{{name}}</p>
        <a href="#" @click="refreshVideo">
            <i class="fas fa-fw fa-redo-alt"></i>
        </a>
        <a href="#" @click="$emit('delete')">
            <i class="fas fa-fw fa-trash"></i>
        </a>
        <a href="#" @click="$emit('edit')">
            <i class="fas fa-fw fa-edit"></i>
        </a>
    </div>
</div>`

export default {
    template: template,
    props: {
        url: String,
        name: String,
        options: {
            type: Object,
            default () {
                return {};
            }
        }
    },
    data() {
        return {
            player: null
        }
    },
    mounted() {
        this.player = videojs(this.$refs.videoPlayer, {
            controls: true,
            preload: "auto",
            autoplay: true,
            muted: true,
            html5: {
                hls: { overrideNative: true }
            },
        });
    },
    beforeDestroy() {
        if (this.player) {
            this.player.dispose()
        }
    },
    methods: {
        refreshVideo() {
            if (this.player) {
                this.player.reset();
                this.player.src({
                    type: 'application/x-mpegURL',
                    src: this.url
                });
            }
        }
    }
}