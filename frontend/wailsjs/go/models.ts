export namespace appconf {
	
	export class AppConfig {
	    access_token?: string;
	    vndb_access_token?: string;
	    theme: string;
	    language: string;
	    ai_provider?: string;
	    ai_base_url?: string;
	    ai_api_key?: string;
	    ai_model?: string;
	    cloud_backup_enabled: boolean;
	    backup_password?: string;
	    backup_user_id?: string;
	    s3_endpoint?: string;
	    s3_region?: string;
	    s3_bucket?: string;
	    s3_access_key?: string;
	    s3_secret_key?: string;
	    cloud_backup_retention?: number;
	    last_db_backup_time?: string;
	    pending_db_restore?: string;
	
	    static createFrom(source: any = {}) {
	        return new AppConfig(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.access_token = source["access_token"];
	        this.vndb_access_token = source["vndb_access_token"];
	        this.theme = source["theme"];
	        this.language = source["language"];
	        this.ai_provider = source["ai_provider"];
	        this.ai_base_url = source["ai_base_url"];
	        this.ai_api_key = source["ai_api_key"];
	        this.ai_model = source["ai_model"];
	        this.cloud_backup_enabled = source["cloud_backup_enabled"];
	        this.backup_password = source["backup_password"];
	        this.backup_user_id = source["backup_user_id"];
	        this.s3_endpoint = source["s3_endpoint"];
	        this.s3_region = source["s3_region"];
	        this.s3_bucket = source["s3_bucket"];
	        this.s3_access_key = source["s3_access_key"];
	        this.s3_secret_key = source["s3_secret_key"];
	        this.cloud_backup_retention = source["cloud_backup_retention"];
	        this.last_db_backup_time = source["last_db_backup_time"];
	        this.pending_db_restore = source["pending_db_restore"];
	    }
	}

}

export namespace enums {
	
	export enum SourceType {
	    LOCAL = "local",
	    BANGUMI = "bangumi",
	    VNDB = "vndb",
	    YMGAL = "ymgal",
	}
	export enum Period {
	    YEAR = "year",
	    MONTH = "month",
	    WEEK = "week",
	}

}

export namespace models {
	
	export class Game {
	    id: string;
	    name: string;
	    cover_url: string;
	    company: string;
	    summary: string;
	    path: string;
	    save_path: string;
	    source_type: enums.SourceType;
	    // Go type: time
	    cached_at: any;
	    source_id: string;
	    // Go type: time
	    created_at: any;
	
	    static createFrom(source: any = {}) {
	        return new Game(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	        this.cover_url = source["cover_url"];
	        this.company = source["company"];
	        this.summary = source["summary"];
	        this.path = source["path"];
	        this.save_path = source["save_path"];
	        this.source_type = source["source_type"];
	        this.cached_at = this.convertValues(source["cached_at"], null);
	        this.source_id = source["source_id"];
	        this.created_at = this.convertValues(source["created_at"], null);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class GameBackup {
	    id: string;
	    game_id: string;
	    backup_path: string;
	    size: number;
	    // Go type: time
	    created_at: any;
	
	    static createFrom(source: any = {}) {
	        return new GameBackup(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.game_id = source["game_id"];
	        this.backup_path = source["backup_path"];
	        this.size = source["size"];
	        this.created_at = this.convertValues(source["created_at"], null);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class User {
	    id: string;
	    // Go type: time
	    created_at: any;
	    default_backup_target: string;
	
	    static createFrom(source: any = {}) {
	        return new User(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.created_at = this.convertValues(source["created_at"], null);
	        this.default_backup_target = source["default_backup_target"];
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}

}

export namespace sql {
	
	export class DB {
	
	
	    static createFrom(source: any = {}) {
	        return new DB(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	
	    }
	}

}

export namespace vo {
	
	export class AISummaryRequest {
	    dimension: string;
	
	    static createFrom(source: any = {}) {
	        return new AISummaryRequest(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.dimension = source["dimension"];
	    }
	}
	export class AISummaryResponse {
	    summary: string;
	    dimension: string;
	
	    static createFrom(source: any = {}) {
	        return new AISummaryResponse(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.summary = source["summary"];
	        this.dimension = source["dimension"];
	    }
	}
	export class CategoryVO {
	    id: string;
	    name: string;
	    is_system: boolean;
	    // Go type: time
	    created_at: any;
	    // Go type: time
	    updated_at: any;
	    game_count: number;
	
	    static createFrom(source: any = {}) {
	        return new CategoryVO(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	        this.is_system = source["is_system"];
	        this.created_at = this.convertValues(source["created_at"], null);
	        this.updated_at = this.convertValues(source["updated_at"], null);
	        this.game_count = source["game_count"];
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class CloudBackupItem {
	    key: string;
	    name: string;
	    size: number;
	    // Go type: time
	    created_at: any;
	
	    static createFrom(source: any = {}) {
	        return new CloudBackupItem(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.key = source["key"];
	        this.name = source["name"];
	        this.size = source["size"];
	        this.created_at = this.convertValues(source["created_at"], null);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class CloudBackupStatus {
	    enabled: boolean;
	    configured: boolean;
	    user_id: string;
	
	    static createFrom(source: any = {}) {
	        return new CloudBackupStatus(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.enabled = source["enabled"];
	        this.configured = source["configured"];
	        this.user_id = source["user_id"];
	    }
	}
	export class DBBackupInfo {
	    path: string;
	    name: string;
	    size: number;
	    // Go type: time
	    created_at: any;
	
	    static createFrom(source: any = {}) {
	        return new DBBackupInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.path = source["path"];
	        this.name = source["name"];
	        this.size = source["size"];
	        this.created_at = this.convertValues(source["created_at"], null);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class DBBackupStatus {
	    last_backup_time: string;
	    backups: DBBackupInfo[];
	
	    static createFrom(source: any = {}) {
	        return new DBBackupStatus(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.last_backup_time = source["last_backup_time"];
	        this.backups = this.convertValues(source["backups"], DBBackupInfo);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class DailyPlayTime {
	    date: string;
	    duration: number;
	
	    static createFrom(source: any = {}) {
	        return new DailyPlayTime(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.date = source["date"];
	        this.duration = source["duration"];
	    }
	}
	export class GameDetailStats {
	    total_play_count: number;
	    total_play_time: number;
	    today_play_time: number;
	    recent_play_history: DailyPlayTime[];
	
	    static createFrom(source: any = {}) {
	        return new GameDetailStats(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.total_play_count = source["total_play_count"];
	        this.total_play_time = source["total_play_time"];
	        this.today_play_time = source["today_play_time"];
	        this.recent_play_history = this.convertValues(source["recent_play_history"], DailyPlayTime);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class GameMetadataFromWebVO {
	    Source: enums.SourceType;
	    Game: models.Game;
	
	    static createFrom(source: any = {}) {
	        return new GameMetadataFromWebVO(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.Source = source["Source"];
	        this.Game = this.convertValues(source["Game"], models.Game);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class GamePlayStats {
	    game_id: string;
	    game_name: string;
	    cover_url: string;
	    total_duration: number;
	
	    static createFrom(source: any = {}) {
	        return new GamePlayStats(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.game_id = source["game_id"];
	        this.game_name = source["game_name"];
	        this.cover_url = source["cover_url"];
	        this.total_duration = source["total_duration"];
	    }
	}
	export class TimePoint {
	    label: string;
	    duration: number;
	
	    static createFrom(source: any = {}) {
	        return new TimePoint(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.label = source["label"];
	        this.duration = source["duration"];
	    }
	}
	export class GameTrendSeries {
	    game_id: string;
	    game_name: string;
	    points: TimePoint[];
	
	    static createFrom(source: any = {}) {
	        return new GameTrendSeries(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.game_id = source["game_id"];
	        this.game_name = source["game_name"];
	        this.points = this.convertValues(source["points"], TimePoint);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class HomePageData {
	    recent_games: models.Game[];
	    recently_added: models.Game[];
	    today_play_time_sec: number;
	    weekly_play_time_sec: number;
	
	    static createFrom(source: any = {}) {
	        return new HomePageData(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.recent_games = this.convertValues(source["recent_games"], models.Game);
	        this.recently_added = this.convertValues(source["recently_added"], models.Game);
	        this.today_play_time_sec = source["today_play_time_sec"];
	        this.weekly_play_time_sec = source["weekly_play_time_sec"];
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class MetadataRequest {
	    source: enums.SourceType;
	    id: string;
	
	    static createFrom(source: any = {}) {
	        return new MetadataRequest(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.source = source["source"];
	        this.id = source["id"];
	    }
	}
	export class PeriodStats {
	    dimension: enums.Period;
	    total_play_count: number;
	    total_play_duration: number;
	    play_time_leaderboard: GamePlayStats[];
	    timeline: TimePoint[];
	    leaderboard_series: GameTrendSeries[];
	
	    static createFrom(source: any = {}) {
	        return new PeriodStats(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.dimension = source["dimension"];
	        this.total_play_count = source["total_play_count"];
	        this.total_play_duration = source["total_play_duration"];
	        this.play_time_leaderboard = this.convertValues(source["play_time_leaderboard"], GamePlayStats);
	        this.timeline = this.convertValues(source["timeline"], TimePoint);
	        this.leaderboard_series = this.convertValues(source["leaderboard_series"], GameTrendSeries);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}

}

